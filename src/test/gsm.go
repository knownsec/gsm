package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"github.com/stianeikeland/go-rpio"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"utils"
)

var config utils.Config

var wxAccessToken utils.WXAccessToken
var baiDuAccessToken utils.BaiDuAccessToken

var taskbus = make(chan utils.PhoneMsg, 100)
var resultbus = make(chan utils.PhoneMsg, 100)

func getStrUnicode(s string) string {
	result := fmt.Sprintf("%U", []rune(s))
	result = strings.ReplaceAll(result, "[", "")
	result = strings.ReplaceAll(result, "]", "")
	result = strings.ReplaceAll(result, "U+", "")
	result = strings.ReplaceAll(result, " ", "")
	return result
}

func executeCmd(cmd string, cmdDict map[string]string, taskbus chan utils.PhoneMsg) {
	var flag string
	var exec_result string
	phone_num_match := `^(\+)?\d+`
	phone_num_rgx := regexp.MustCompile(phone_num_match)
	if _, ok := cmdDict[cmd]; ok {
		if strings.HasPrefix(cmdDict[cmd], "http://") || strings.HasPrefix(cmdDict[cmd], "https://") {
			flag = "http"
		}
	}

	if strings.HasPrefix(cmd, "cmd::") {
		flag = "cmd"
	}
	if strings.HasPrefix(cmd, "dial::") {
		flag = "dial"
	}
	if strings.HasPrefix(cmd, "sms::") {
		flag = "sms"
	}

	switch flag {
	case "http":
		resp, err := http.Get(cmdDict[cmd])
		if err != nil {
			exec_result = err.Error()
			break
		}

		defer resp.Body.Close()

		body, _ := ioutil.ReadAll(resp.Body)
		exec_result = string(body[:])
	case "cmd":
		var err error
		var out []byte
		tmpcmds := strings.Replace(cmd, "cmd::", "", -1)
		if strings.Contains(tmpcmds, " ") {
			tmp2cmds := strings.Split(tmpcmds, " ")
			out, err = exec.Command(tmp2cmds[0], tmp2cmds[1:]...).Output()
		} else {
			out, err = exec.Command(tmpcmds).Output()
		}
		if err != nil {
			exec_result = err.Error()
		} else {
			if len(out) > 0 {
				exec_result = string(out)
			} else {
				exec_result = tmpcmds + " 执行成功."
			}
		}
	case "dial":
		phone_cmd := strings.Replace(cmd, "dial::", "", -1)
		if phone_num_rgx.MatchString(phone_cmd) {
			phoneMsg := utils.PhoneMsg{CmdDelay: 1}
			if strings.ToLower(phone_cmd) == "ath" {
				phoneMsg.ATCmd = []byte(utils.CMD_ATH + utils.CMD_LF_CR)
			} else {
				phoneMsg.ATCmd = []byte(utils.CMD_ATD + phone_cmd + ";" + utils.CMD_LF_CR)
			}
			taskbus <- phoneMsg
		} else {
			exec_result = "抱歉，手机号码有误"
		}
	case "sms":
		infos := strings.SplitN(strings.ReplaceAll(cmd, "sms::", ""), "::", 2)
		phonecode := getStrUnicode(infos[0])
		smsbody := getStrUnicode(infos[1])

		if phone_num_rgx.MatchString(phonecode) {
			set_message_format_cmd := []byte(utils.CMD_CMGF + utils.CMD_LF_CR)
			smphoneMsg := utils.PhoneMsg{CmdDelay: 1}
			smphoneMsg.ATCmd = set_message_format_cmd
			taskbus <- smphoneMsg

			ucs2cmd := []byte(utils.CMD_CSCS_UCS2 + utils.CMD_LF_CR)
			setucs2 := utils.PhoneMsg{CmdDelay: 1}
			setucs2.ATCmd = ucs2cmd
			taskbus <- setucs2

			csmpcmd := []byte(utils.CMD_CSMP + utils.CMD_LF_CR)
			setcsmp := utils.PhoneMsg{CmdDelay: 1}
			setcsmp.ATCmd = csmpcmd
			taskbus <- setcsmp

			phoneMsg := utils.PhoneMsg{CmdDelay: 2}

			phoneMsg.ATCmd = []byte(utils.CMD_CMGS + phonecode + "\":::" + smsbody + utils.CMD_CTRL_Z)
			taskbus <- phoneMsg
		} else {
			exec_result = "抱歉，手机号码有误"
		}

	default:
		exec_result = "抱歉，未查到此: " + cmd + " 指令"
	}
	utils.SendWXMsg(exec_result, config.WxAgentid, config.WxUser, wxAccessToken.AccessToken)
}

func readFileToMap(filename string) map[string]string {
	result := make(map[string]string)
	var k string
	var v string
	body, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Fatal("read file error %s", err)
		return nil
	}
	for _, line := range strings.Split(string(body[:]), "\n") {
		if strings.Contains(line, "::") {
			str_array := strings.SplitN(line, "::", 2)
			k, v = str_array[0], str_array[1]
			result[k] = v
		}
	}
	return result
}

func process_command(command_bus chan string) {
	for {
		command := <-command_bus
		command = strings.Replace(command, "。", "", -1)
		command = strings.Replace(command, "，", "", -1)
		command = strings.Replace(command, ",", "", -1)
		cmdDict := readFileToMap(config.CMDFile)
		executeCmd(command, cmdDict, taskbus)
	}
}

func get_info(msg_send chan *utils.MSG) {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	httpclient := &http.Client{
		Transport: tr,
	}
	for {
		msg := &utils.MSG{}
		tmp_timestamp, tmp_nonce := msg.MakeTimestampNonce()
		msg.Timestamp = strconv.FormatInt(tmp_timestamp, 10)
		msg.Nonce = strconv.FormatUint(uint64(tmp_nonce), 10)
		garblebytes := msg.MakeGarbleByte(config.SecretWord)
		encryptstr := msg.EncryptStr(string(garblebytes[:]), config.AESKEY)
		msg.EchoStr = string(encryptstr[:])
		msg.MsgSignature = msg.MakeMsgSignature(config.TOKEN)
		url := fmt.Sprintf("%s?msg_signature=%s&timestamp=%s&nonce=%s&echostr=%s", config.TargetURL, msg.MsgSignature, msg.Timestamp, msg.Nonce, msg.EchoStr)
		resp, err := httpclient.Get(url)
		msg.CheckErr(err)
		resp_body, _ := ioutil.ReadAll(resp.Body)
		defer resp.Body.Close()
		if resp.StatusCode == int(200) && len(resp_body) > 0 && bytes.Compare(resp_body, []byte(config.FakeBody)) != 0 {
			xml.Unmarshal(resp_body, msg)
			xml.Unmarshal(msg.DecryptStr(msg.Encrypt, config.AESKEY), msg)
			msg_send <- msg
		}
		time.Sleep(time.Duration(1) * time.Second)
	}
}

func decrypt_message(msg_send chan *utils.MSG, command_bus chan string) {
	msg := &utils.MSG{}
	bdVoice := &utils.BaiDuVoice{Rate: 8000, Channel: 1, DevPid: 1537, Cuid: config.BaiDuYuYingCuid}
	for {
		msg = <-msg_send
		switch msg.MsgType {
		case "voice":
			wx_body, wx_len := utils.GetWXVoiceBody(wxAccessToken.AccessToken, msg.MediaId)
			bdVoice.Token = baiDuAccessToken.AccessToken
			bdVoice.Len = wx_len
			bdVoice.Format = msg.Format
			bdVoice.Speech = wx_body
			bdVoiceResult := utils.GetBaiduVoiceResult(bdVoice)
			if bdVoiceResult.ErrNo == 0 {
				command_bus <- bdVoiceResult.Result[0]
			}
		case "text":
			if len(wxAccessToken.AccessToken) > 0 {
				command_bus <- msg.Content
			}
		default:
			fmt.Println(msg.MsgType)
		}
	}
}

func get_temp(filename string) (float32, error) {
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		return 0.0, err
	}

	tmp, err := strconv.Atoi(strings.TrimSpace(string(content[:])))
	if err != nil {
		return 0.0, err
	}

	cpu_temp := float32(tmp) / 1000
	return cpu_temp, nil
}

func control_cpu_fan(cpu_temp_file string, timeout time.Duration, cpu_temp float32) {
	err := rpio.Open()
	utils.CheckErr(err)
	defer rpio.Close()
	fan_contrl := rpio.Pin(config.CPUFanConPin)
	fan_contrl.Output()
	for {
		temp, err := get_temp(cpu_temp_file)
		utils.CheckErr(err)
		diff_num := int(temp - cpu_temp)

		if diff_num > 2 {
			fan_contrl.High()
		}

		if diff_num < -5 {
			fan_contrl.Low()
		}
		time.Sleep(timeout * time.Minute)
	}

}

func main() {
	if len(os.Args) != 2 {
		fmt.Printf("Usage: %s config.json\n", os.Args[0])
		return
	}
	file_body, err := ioutil.ReadFile(os.Args[1])
	if err != nil {
		panic(err)
	}
	json.Unmarshal(file_body, &config)

	var wg sync.WaitGroup
	wg.Add(1)

	recvmsg_bus := make(chan *utils.MSG, 10)
	command_bus := make(chan string, 10)

	go utils.GetWXAccessToken(&wxAccessToken, config.WxCorpid, config.WxCorpSecret)
	go utils.GetBaiDuYuYingAccessToken(&baiDuAccessToken, config.BaiDuYuYingKey, config.BaiDuYuYingSecret)

	go get_info(recvmsg_bus)
	go decrypt_message(recvmsg_bus, command_bus)
	go process_command(command_bus)

	if config.CheckCpuTemp {
		go control_cpu_fan(config.CPUTempFile, time.Duration(config.TempInterval), config.CPUFanStart)
	}

	go utils.AddCycleATCmd(taskbus)
	go utils.ExecATCmd(taskbus, resultbus, config)
	go utils.ProcessATcmdResult(resultbus, &config, &wxAccessToken.AccessToken)

	wg.Wait()
}
