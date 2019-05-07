package utils

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/jacobsa/go-serial/serial"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
	"io/ioutil"
	"log"
	"net/http"
	"net/smtp"
	"regexp"
	"strings"
	"time"
)

const (
	CMD_COPS      string = "AT+COPS?"
	CMD_CMGF      string = "AT+CMGF=1"
	CMD_CSCS_UCS2 string = "AT+CSCS=\"UCS2\"" //设置编码
	CMD_CSCS_GSM  string = "AT+CSCS=\"GSM\""  //设置编码
	CMD_CSMP      string = "AT+CSMP=17,71,0,8"
	CMD_CMGFZ     string = "AT+CMGF=0"
	CMD_CMGL_ALL  string = "AT+CMGL=\"REC UNREAD\"" //获取所有未读短信
	CMD_CMGDA_ALL string = "AT+CMGD=1,3"            //SIM900A 这个指令为：AT+CMGDA="DEL ALL" 删除已读短信
	CMD_CMGS      string = "AT+CMGS=\""             //发送短信指令 后跟手机号码
	CMD_ATD       string = "ATD"                    //呼叫号码
	CMD_ATH       string = "ATH"                    //挂机
	CMD_CTRL_Z    string = "\x1A"
	CMD_LF_CR     string = "\r\n"
	CMD_LF        string = "\r"
	CACHE_SIZE    int    = 1024 * 8
)

type SendMsgResp struct {
	ErrorCode   int    `json:"errorcode"`
	Errmsg      string `json:"errmsg"`
	Invaliduser string `json:"invaliduser"`
}

type PhoneMsg struct {
	ATCmd     []byte
	CmdDirect string
	Result    string
	SendMSG   string
	CmdDelay  uint
}

func CheckErr(err error) {
	if err != nil {
		log.Fatalf(err.Error())
	}
}
func Utf8ToUcs2(in string) (string, error) {
	r := bytes.NewReader([]byte(in))
	t := transform.NewReader(r, unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM).NewEncoder()) //UTF-16 bigendian, no-bom
	out, err := ioutil.ReadAll(t)
	if err != nil {
		return "", err
	}
	hexStr := fmt.Sprintf("%X", out)
	return hexStr, nil
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

func IsUcs(body string) bool {
	reg := regexp.MustCompile(`[^[:xdigit:]]`)
	idx := reg.FindStringIndex(body)
	if len(idx) == 0 {
		return true
	} else {
		return false
	}
}
func ExecATCmd(input chan PhoneMsg, result chan PhoneMsg, config Config) {
	options := serial.OpenOptions{
		PortName:        config.Device,
		BaudRate:        config.Baudrate,
		DataBits:        8,
		StopBits:        1,
		MinimumReadSize: 4,
	}

	port, err := serial.Open(options)
	CheckErr(err)
	defer port.Close()
	info_cache := make([]byte, CACHE_SIZE)
	for {
		execphonemsg := <-input
		if strings.HasPrefix(string(execphonemsg.ATCmd[:]), CMD_CMGS) {
			bodys := strings.SplitN(string(execphonemsg.ATCmd[:]), ":::", 2)
			tmp_control := string(bodys[0]) + CMD_LF
			_, err = port.Write([]byte(tmp_control))
			time.Sleep(time.Duration(1) * time.Second)
			CheckErr(err)
			port.Write([]byte(bodys[1]))
			info_cache = make([]byte, CACHE_SIZE)
			port.Read(info_cache)
			execphonemsg.Result = string(info_cache[:])
			result <- execphonemsg
		} else {
			_, err = port.Write(execphonemsg.ATCmd)
			CheckErr(err)
			time.Sleep(time.Duration(1) * time.Second)
			port.Read(info_cache)
			execphonemsg.Result = string(info_cache[:])
			if strings.HasPrefix(execphonemsg.Result, ">") {
				port.Write([]byte(CMD_CTRL_Z))
			}
			result <- execphonemsg
			info_cache = make([]byte, CACHE_SIZE)
		}
	}
}
func AddCycleATCmd(input chan PhoneMsg) {
	read_all_messages_cmd := []byte(CMD_CMGL_ALL + CMD_LF_CR)
	clean_all_read_messages_cmd := []byte(CMD_CMGDA_ALL + CMD_LF_CR)
	i := 0
	for {
		raphoneMsg := PhoneMsg{CmdDelay: 1}
		raphoneMsg.ATCmd = read_all_messages_cmd
		input <- raphoneMsg

		//定向投毒
		if i == 12 {
			carphoneMsg := PhoneMsg{CmdDelay: 1}
			carphoneMsg.ATCmd = clean_all_read_messages_cmd
			input <- carphoneMsg
			i = 0
		}

		time.Sleep(time.Duration(5) * time.Second)
		i += 1
	}
}

func ProcessATcmdResult(result chan PhoneMsg, config *Config, token *string) {
	for {
		phoneMsg := <-result
		//可以在这里对不同指令的处理结果
		if strings.HasPrefix(string(phoneMsg.ATCmd), CMD_CMGL_ALL) && strings.Contains(phoneMsg.Result, "OK") {
			msgs := strings.Split(phoneMsg.Result, CMD_LF_CR)
			for i, m := range msgs {
				if strings.HasPrefix(m, "+CMGL:") && strings.Contains(m, "UNREAD") {
					tmp_info := strings.Split(msgs[i], ",\"")
					src := strings.ReplaceAll(tmp_info[2], "\"", "")
					src = strings.ReplaceAll(src, ",", "")
					tmpsrc, _ := hex.DecodeString(src)
					phonenum, _ := Ucs2ToUtf8(string(tmpsrc))
					t := strings.Replace(tmp_info[3], "\"", "", -1) //SIM900A 格式应改为：tmp_info[4]
					if len(phoneMsg.SendMSG) != 0 {
						phoneMsg.SendMSG += "\n"
					}
					phoneMsg.SendMSG += "来源: " + phonenum + " 时间: " + t + "\n"
					if IsUcs(msgs[i+1]) {
						dat, _ := hex.DecodeString(msgs[i+1])
						tmpmsg, _ := Ucs2ToUtf8(string(dat))
						phoneMsg.SendMSG += tmpmsg
					} else {
						phoneMsg.SendMSG += msgs[i+1]
					}
				}
			}
		}
		if strings.HasPrefix(string(phoneMsg.ATCmd), CMD_ATD) {
			if strings.Contains(phoneMsg.Result, "OK") {
				phoneMsg.SendMSG = "拨打电话成功"
			} else {
				phoneMsg.SendMSG = "拨打电话失败"
			}
		}

		if strings.HasPrefix(string(phoneMsg.ATCmd), CMD_CMGS) {
			if strings.Contains(phoneMsg.Result, "ERROR") {
				phoneMsg.SendMSG = "发送短信失败"
			} else {
				phoneMsg.SendMSG = "发送短信成功"
			}
		}

		if config.SendWX && len(phoneMsg.SendMSG) > 0 {
			SendWXMsg(phoneMsg.SendMSG, config.WxAgentid, config.WxUser, *token)
		}
		if config.SendMail && len(phoneMsg.SendMSG) > 0 {
			SendMail(phoneMsg.SendMSG, "来短信了", *config)
		}
	}
}

func SendWXMsg(msg_body string, agentid uint, touser string, accesstoken string) {

	tmpwx := strings.Replace(msg_body, "<h3>", "", -1)
	msg_body = strings.Replace(tmpwx, "</h3>", "", -1)

	send_msg_url := "https://qyapi.weixin.qq.com/cgi-bin/message/send?access_token=" + accesstoken
	msg_content := map[string]string{"content": msg_body}
	send_msg_body := map[string]interface{}{"msgtype": "text", "touser": touser, "agentid": agentid, "text": msg_content}

	jsonvals, _ := json.Marshal(send_msg_body)
	sm_resp, err := http.Post(send_msg_url, "application/json", bytes.NewBuffer(jsonvals))
	CheckErr(err)
	defer sm_resp.Body.Close()
	var smr SendMsgResp
	smrb, _ := ioutil.ReadAll(sm_resp.Body)
	json.Unmarshal(smrb, &smr)
	if smr.ErrorCode != 0 {
		log.Println(smr.Errmsg)
	}
}

func SendMail(body string, subject string, config Config) {
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

	CheckErr(err)
}
