package utils

type Config struct {
	Device            string  `json:"device"`
	Baudrate          uint    `json:"baudrate"`
	ErrorSleep        uint    `json:"sleep"`
	CheckCpuTemp      bool    `json:"checkcputemp"`
	TempInterval      uint    `json:"tempinterval"`
	CPUFanStart       float32 `json:"cpufanstart"`
	CPUFanConPin      uint    `json:"cpufanconpin"`
	CPUTempFile       string  `json:"cputempfile"`
	SendWX            bool    `json:"sendwx"`
	WxCorpid          string  `json:"wxcorpid"`
	WxAgentid         uint    `json:"wxagentid"`
	WxUser            string  `json:"wxuser"`
	WxCorpSecret      string  `json:"wxcorpsecret"`
	SendMail          bool    `json:"sendmail"`
	MailFrom          string  `json:"mailfrom"`
	MailTo            string  `json:"mailto"`
	MailPass          string  `json:"mailpass"`
	MailServer        string  `json:"mailserver"`
	MailServerPort    uint    `json:"mailserverport"`
	BaiDuYuYingKey    string  `json:"bdyykey"`
	BaiDuYuYingSecret string  `json:"bdyysecret"`
	BaiDuYuYingCuid   string  `json:"cuid"`

	Port         uint16 `json:"port"`
	SSL          bool   `json:"ssl"`
	AESKEY       string `json:"aeskey"`
	TOKEN        string `json:"token"`
	HeaderServer string `json:"headerserver"`
	FakeBody     string `json:"fakebody"`
	SecretWord   string `json:"secretword"`
	CheckURL     string `json:"checkurl"`
	TargetURL    string `json:"targeturl"`
	CertFile     string `json:"certfile"`
	KeyFile      string `json:"keyfile"`
	CMDFile      string `json:"cmdfile"`
}
