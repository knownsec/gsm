package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/jacobsa/go-serial/serial"
	"github.com/stianeikeland/go-rpio"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
	"io/ioutil"
	"log"
	"net/http"
	"net/smtp"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	CMD_COPS      string = "AT+COPS?"
	CMD_CMGF      string = "AT+CMGF=1"
	CMD_CMGL_ALL  string = "AT+CMGL=\"ALL\""
	CMD_CMGDA_ALL string = "AT+CMGD=1,4" //SIM900A 这个指令为：AT+CMGDA="DEL ALL"
	CMD_CTRL_Z    string = "\x1A"
	CMD_LF_CR     string = "\r\n"
	CACHE_SIZE    int    = 4 * 1024
	OPERATOR_FLAG string = "CHINA MOBILE"
	ERROR_FLAG    string = "ERROR"
	CPU_TEMP_FILE string = "/sys/class/thermal/thermal_zone0/temp"
	CMD_SEND_MSG  string = "AT+CMGS=\"10086\"" + CMD_LF_CR + "YE" + CMD_LF_CR + CMD_CTRL_Z
)

type Config struct {
	Device         string  `json:"device"`
	Baudrate       uint    `json:"baudrate"`
	ErrorSleep     uint    `json:"sleep"`
	CheckCpuTemp   bool    `json:"checkcputemp"`
	TempInterval   uint    `json:"tempinterval"`
	CPUFanStart    float32 `json:"cpufanstart"`
	CPUFanConPin   uint    `json:"cpufanconpin"`
	CPUTempFile    string  `json:"cputempfile"`
	SendWX         bool    `json:"sendwx"`
	WxCorpid       string  `json:"wxcorpid"`
	WxAgentid      uint    `json:"wxagentid"`
	WxUser         string  `json:"wxuser"`
	WxCorpSecret   string  `json:"wxcorpsecret"`
	SendMail       bool    `json:"sendmail"`
	MailFrom       string  `json:"mailfrom"`
	MailTo         string  `json:"mailto"`
	MailPass       string  `json:"mailpass"`
	MailServer     string  `json:"mailserver"`
	MailServerPort uint    `json:"mailserverport"`
}

type AccessToken struct {
	ErrorCode    int    `json:"errorcode"`
	Errmsg       string `json:"errmsg"`
	Access_token string `json:"access_token"`
	Expires_in   int    `json:"expires_in"`
}

type SendMsgResp struct {
	ErrorCode   int    `json:"errorcode"`
	Errmsg      string `json:"errmsg"`
	Invaliduser string `json:"invaliduser"`
}

var config Config

func check_err(err error) {
	if err != nil {
		log.Fatalf("serial.Open: %v", err)
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

func send_wx_msg(msg_body string, corpid string, corpsecret string, agentid uint, touser string) {
	var access_token_resp AccessToken
	get_honeypot_access_tokey := "https://qyapi.weixin.qq.com/cgi-bin/gettoken?corpid=" + corpid + "&&corpsecret=" + corpsecret
	resp, err := http.Get(get_honeypot_access_tokey)
	check_err(err)

	tmpwx := strings.Replace(msg_body, "<h3>", "", -1)
	msg_body = strings.Replace(tmpwx, "</h3>", "", -1)

	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)
	json.Unmarshal(body, &access_token_resp)
	if access_token_resp.ErrorCode != 0 {
		log.Println(access_token_resp.Errmsg)
	}

	send_msg_url := "https://qyapi.weixin.qq.com/cgi-bin/message/send?access_token=" + access_token_resp.Access_token
	msg_content := map[string]string{"content": msg_body}
	send_msg_body := map[string]interface{}{"msgtype": "text", "touser": touser, "agentid": agentid, "text": msg_content}

	jsonvals, _ := json.Marshal(send_msg_body)
	sm_resp, err := http.Post(send_msg_url, "application/json", bytes.NewBuffer(jsonvals))
	check_err(err)
	defer sm_resp.Body.Close()
	var smr SendMsgResp
	smrb, _ := ioutil.ReadAll(sm_resp.Body)
	json.Unmarshal(smrb, &smr)
	if smr.ErrorCode != 0 {
		log.Println(smr.Errmsg)
	}
}

func send_mail(body string, subject string) {
	from := config.MailFrom
	pass := config.MailPass
	to := config.MailTo

	msg := "From: " + from + "\r\n" +
		"To: " + to + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"Content-Type: text/html; charset=UTF-8\r\n\r\n" +
		body

	addr := fmt.Sprintf("%s:%d", config.MailServer, config.MailServerPort)
	err := smtp.SendMail(addr,
		smtp.PlainAuth("", from, pass, config.MailServer),
		from, []string{to}, []byte(msg))

	check_err(err)
}

func control_cpu_fan(cpu_temp_file string, timeout time.Duration, cpu_temp float32) {
	err := rpio.Open()
	check_err(err)
	defer rpio.Close()
	fan_contrl := rpio.Pin(config.CPUFanConPin)
	fan_contrl.Output()
	for {
		temp, err := get_temp(cpu_temp_file)
		check_err(err)
		diff_num := int(temp - cpu_temp)

		if diff_num > 2 {
			fan_contrl.High()
		}

		if diff_num < -5 {
			fan_contrl.Low()
		}
		time.Sleep(timeout * time.Second)
	}

}

func Ucs2ToUtf8(in string) (string, error) {
	r := bytes.NewReader([]byte(in))
	t := transform.NewReader(r, unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM).NewDecoder()) //UTF-16 bigendian, no-bom
	out, err := ioutil.ReadAll(t)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func exec_cmd(cmd_str []byte, i time.Duration) string {
	options := serial.OpenOptions{
		PortName:        config.Device,
		BaudRate:        config.Baudrate,
		DataBits:        8,
		StopBits:        1,
		MinimumReadSize: 4,
	}

	port, err := serial.Open(options)
	check_err(err)
	defer port.Close()
	info_cache := make([]byte, CACHE_SIZE)
	_, err = port.Write(cmd_str)
	check_err(err)
	time.Sleep(i * time.Second)
	port.Read(info_cache)
	time.Sleep(i * time.Second)
	return string(info_cache[:])
}

func isucs(body string) bool {
	reg := regexp.MustCompile(`[^[:xdigit:]]`)
	idx := reg.FindStringIndex(body)
	if len(idx) == 0 {
		return true
	} else {
		return false
	}
}

func parse_msg(msg string) string {
	msgs := strings.Split(msg, "\r\n")
	result_msg := ""
	for i, m := range msgs {
		if strings.HasPrefix(m, "+CMGL:") && strings.Contains(m, "UNREAD") {
			tmp_info := strings.Split(msgs[i], ",\"")
			src := strings.Replace(tmp_info[2], "\"", "", -1)
			t := strings.Replace(tmp_info[3], "\"", "", -1) //SIM900A 格式应改为：tmp_info[4]
			result_msg += "来源: " + src + " 时间: " + t + "\n"
			if isucs(msgs[i+1]) {
				dat, _ := hex.DecodeString(msgs[i+1])
				tmpmsg, _ := Ucs2ToUtf8(string(dat))
				result_msg += tmpmsg
			} else {
				result_msg += msgs[i+1]
			}
		}
	}
	return result_msg
}

func check_info(msg string) bool {
	if strings.Index(msg, ERROR_FLAG) != -1 {
		return false
	}
	return true
}

func get_message(msg_send chan string) {
	read_all_messages_cmd := []byte(CMD_CMGL_ALL + CMD_LF_CR)
	set_message_format_cmd := []byte(CMD_CMGF + CMD_LF_CR)
	clean_all_message__cmd := []byte(CMD_CMGDA_ALL + CMD_LF_CR)
	recv_message := ""
	for {
		recv_message = ""
		recv_message += exec_cmd(set_message_format_cmd, 1)
		recv_message += exec_cmd(read_all_messages_cmd, 1)
		if strings.Contains(recv_message, "ERROR") {
			time.Sleep(time.Duration(config.ErrorSleep) * time.Second)
			continue
		}
		if strings.Contains(recv_message, "READ") {
			msg_send <- recv_message
			recv_message += exec_cmd(clean_all_message__cmd, 1)
		}
	}

}

func process_message(msg_send chan string) {
	msg := ""
	for {
		msg = <-msg_send
		if check_info(msg) {
			result_msg := parse_msg(msg)
			if len(result_msg) > 0 {
				if config.SendMail {
					go send_mail(result_msg, "来短信啦")
				}
				if config.SendWX {
					go send_wx_msg(result_msg, config.WxCorpid, config.WxCorpSecret, config.WxAgentid, config.WxUser)

				}
			}
		} else {
			msg = strings.Replace(msg, "\r\n", "\n", -1)
			log.Println(msg)
		}
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

	msg_bus := make(chan string, 100)
	var wg sync.WaitGroup
	wg.Add(1)

	if config.CheckCpuTemp {
		go control_cpu_fan(config.CPUTempFile, 600, config.CPUFanStart)
	}

	go get_message(msg_bus)

	go process_message(msg_bus)

	wg.Wait()
}
