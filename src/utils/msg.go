package utils

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"sort"
	"strings"
	"time"
)

type WXAccessToken struct {
	ErrorCode   int    `json:"errorcode"`
	Errmsg      string `json:"errmsg"`
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

type BaiDuAccessToken struct {
	AccessToken   string `json:"access_token"`
	ExpiresIn     uint32 `json:"expires_in"`
	RefreshToken  string `json:"refresh_token"`
	Scope         string `json:"scope"`
	SessionKey    string `json:"session_key"`
	SessionSecret string `json:"session_secret"`
}

type MSG struct {
	XMLName      xml.Name `xml:"xml"`
	ToUserName   string   `xml:"ToUserName,CDATA"`
	Encrypt      string   `xml:"Encrypt,CDATA"`
	AgentID      string   `xml:"AgentID,CDATA"`
	MsgType      string   `xml:"MsgType,CDATA"`
	MediaId      string   `xml:"MediaId,CDATA"`
	Content      string   `xml:"Content,CDATA"`
	Format       string   `xml:"Format,CDATA"`
	MsgSignature string
	Timestamp    string
	Nonce        string
	EchoStr      string
}

type BaiDuVoice struct {
	Format  string `json:"format"`
	Rate    int    `json:"rate"`
	Channel int    `json:"channel"`
	Cuid    string `json:"cuid"`
	DevPid  int    `json:"dev_pid"`
	Token   string `json:"token"`
	Speech  string `json:"speech"`
	Len     int    `json:"len"`
}

type BaiDuVoiceResult struct {
	ErrNo    int      `json:"err_no"`
	ErrMsg   string   `json:"err_msg"`
	CorpusNo string   `json:"corpus_no"`
	SN       string   `json:"sn"`
	Result   []string `json:"result"`
}

func (msg *MSG) CheckErr(err error) {
	if err != nil {
		log.Printf("error is: %v", err)
	}
}

func (msg *MSG) PKCS7Padding(ciphertext []byte, blockSize int) []byte {
	padding := blockSize - len(ciphertext)%blockSize
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(ciphertext, padtext...)
}

func (msg *MSG) DecryptStr(str string, key string) []byte {
	aeskey, err := base64.StdEncoding.DecodeString(key + "=")
	msg.CheckErr(err)
	iv := aeskey[:16]
	tmpstr, err := base64.StdEncoding.DecodeString(str)
	block, err := aes.NewCipher(aeskey)
	msg.CheckErr(err)
	blockSize := block.BlockSize()
	blockMode := cipher.NewCBCDecrypter(block, iv)
	origData := make([]byte, len(tmpstr))
	blockMode.CryptBlocks(origData, []byte(tmpstr))

	origData = msg.PKCS7Padding(origData, blockSize)
	content := origData[16:]
	msg_len := binary.BigEndian.Uint32(content[:4])
	return content[4 : msg_len+4]
}

func (msg *MSG) EncryptStr(str string, key string) []byte {
	aeskey, err := base64.StdEncoding.DecodeString(key + "=")
	msg.CheckErr(err)
	iv := aeskey[:16]
	block, err := aes.NewCipher(aeskey)
	msg.CheckErr(err)
	blockSize := block.BlockSize()
	pad_msg := msg.PKCS7Padding([]byte(str), blockSize)
	blockMode := cipher.NewCBCEncrypter(block, iv)
	origData := make([]byte, len(pad_msg))
	blockMode.CryptBlocks(origData, pad_msg)

	base64_msg := make([]byte, base64.StdEncoding.EncodedLen(len(origData)))
	base64.StdEncoding.Encode(base64_msg, origData)

	return base64_msg
}

func (msg *MSG) MakeMsgSignature(token string) string {
	h := sha1.New()
	keys := make([]string, 10)
	keys = append(keys, token)
	keys = append(keys, msg.Timestamp)
	keys = append(keys, msg.Nonce)
	keys = append(keys, msg.EchoStr)
	keys = append(keys, msg.Encrypt)
	sort.Strings(keys)
	check_str := strings.Join(keys, "")
	io.WriteString(h, check_str)
	res := h.Sum(nil)
	return fmt.Sprintf("%x", res)
}
func (msg *MSG) MakeTimestampNonce() (int64, uint64) {
	timestamp := time.Now().Unix()
	rand.Seed(timestamp)
	nonce := rand.Uint64()
	return timestamp, nonce
}

func (msg *MSG) MakeGarbleByte(basestr string) []byte {
	var buf = make([]byte, 4)
	randbyte := msg.GetRandomString(16)
	pandbyte := msg.GetRandomString(10)
	msg_len := len(basestr)
	binary.BigEndian.PutUint32(buf, uint32(msg_len))
	tmp_bytesarray := [][]byte{randbyte, buf, []byte(basestr), pandbyte}
	return bytes.Join(tmp_bytesarray, []byte(""))
}

func (msg *MSG) GetRandomString(maxlen uint32) []byte {
	str := "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	bytes_str := []byte(str)
	result := []byte{}
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := uint32(0); i < maxlen; i++ {
		result = append(result, bytes_str[r.Intn(len(bytes_str))])
	}
	return result
}
func (msg *MSG) CreateMessage(secretkey string) []byte {
	randomstr_16 := msg.GetRandomString(16)
	msg_str_len := len([]byte(secretkey))
	var buf = make([]byte, 8)
	binary.BigEndian.PutUint32(buf, uint32(msg_str_len))
	msg_str := fmt.Sprintf("%s%d%s", randomstr_16, buf, secretkey)
	return []byte(msg_str)
}
func (msg *MSG) Verify(token string) bool {
	tmpstr := msg.MakeMsgSignature(token)
	if strings.Compare(msg.MsgSignature, tmpstr) == 0 {
		return true
	}
	return false
}

func GetWXAccessToken(access_token_resp *WXAccessToken, corpid string, corpsecret string) {
	wx_access_tokey := "https://qyapi.weixin.qq.com/cgi-bin/gettoken?corpid=" + corpid + "&&corpsecret=" + corpsecret
	for {
		resp, err := http.Get(wx_access_tokey)
		if err != nil {
			log.Println(err)
		}

		defer resp.Body.Close()

		body, _ := ioutil.ReadAll(resp.Body)
		json.Unmarshal(body, access_token_resp)
		if access_token_resp.ErrorCode != 0 {
			log.Println(access_token_resp.Errmsg)
		}
		time.Sleep(time.Duration(access_token_resp.ExpiresIn-100) * time.Second)
	}

}

func GetBaiDuYuYingAccessToken(access_token_resp *BaiDuAccessToken, apikey string, secretkey string) {
	baidu_oauth := fmt.Sprintf("https://openapi.baidu.com/oauth/2.0/token?grant_type=client_credentials&client_id=%s&client_secret=%s", apikey, secretkey)
	for {
		resp, err := http.Get(baidu_oauth)
		if err != nil {
			log.Println(err)
		}

		defer resp.Body.Close()

		body, _ := ioutil.ReadAll(resp.Body)
		json.Unmarshal(body, access_token_resp)
		time.Sleep(time.Duration(access_token_resp.ExpiresIn-100) * time.Second)
	}

}

func GetWXVoiceBody(wxtoken string, wxmediaid string) (string, int) {
	wx_media_url := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/media/get?access_token=%s&media_id=%s", wxtoken, wxmediaid)
	resp, err := http.Get(wx_media_url)
	if err != nil {
		log.Println(err)
	}

	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)
	return base64.StdEncoding.EncodeToString(body), len(body)
}

func GetBaiduVoiceResult(voice *BaiDuVoice) BaiDuVoiceResult {
	var result BaiDuVoiceResult
	post_data, err := json.Marshal(voice)
	if err != nil {
		log.Println(err)
	}
	resp, err := http.Post("http://vop.baidu.com/server_api", "application/json", bytes.NewReader(post_data))
	if err != nil {
		log.Println(err)
	}

	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)
	err = json.Unmarshal(body, &result)
	if err != nil {
		log.Println(err)
	}
	return result
}
