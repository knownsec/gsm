
## **硬件短信转发小工具**

这是利用树莓派结合硬件模块，用来规避骚扰电话和信息泄漏的一个小工具.

需要的硬件如下：

* 树莓派    淘宝价格：220 RMB

---

* 乌金甲外壳 铝合金外壳带双风扇  淘宝价格：60 RMB（不是必须）
* 直插三极管NPN SS8050  0.05 RMB （不是必须）

---
* GSM900A  淘宝价格：40-50 RMB
* CH340    淘宝价格：5-10  RMB

**上面两件可以替换为**

* 移远EC20 淘宝价格：150-200 RMB
* 4G模块Mini-PCIE转USB转接板 淘宝价格：20RMB
* 4G天线           淘宝价格：5RMB

**IoT开发模块**

* ESP8266(IoT)      淘宝价格: 20RMB
* 温湿度传感器DHT11   淘宝价格: 5-10RMB

---

### **如何安装**

```
$ sudo apt -y install golang minicom
$ mkdir gopath
$ export GOPATH=./gopath
$ git clone https://github.com/rungobier/gsm.git
$ cd gsm
$ ./install init  #编译环境环境初始化
$ ./install arm   #编译出树莓派版本
$ scp ./bin/gsm-arm <树莓派实际路径下>
$ scp ./src/config.json <树莓派实际路径下>
【树莓派系统下操作】
$ ./gsm-arm config.json
```
其中，项目中需要一些相应的库，但是因为众所周知的原因，在国内很难把它们下载回来，所以我把它们打包成了项目里面的vendor.tar.gz ，在执行初始化的时候展开。

如果需要判定是否是硬件模块的原因，可以使用 `minicom -D /dev/ttyUSB3 -b 115200`，在其中的界面当中执行 AT指令进行指令测试判定。





### **如何配置**
针对config.json的配置文件信息说明如下:

```
{
  "device": "/dev/ttyUSB3",  //短信接收硬件所对应的设备号 SIM900A加CH340默认为 /dev/ttyUSB0
  "baudrate": 115200,    //短信接收硬件设备通讯频率    SIM900A 应该为9600
  "sleep": 5,            //出现错误时的休眠时间
  "sendmail": false,     //是否以发送邮件方式推送收到的短消息
  "mailfrom": "12345678@qq.com",  //发送邮箱账号
  "mailto": "12345678@qq.com",    //接收邮箱账号
  "mailpass": "alksdjfiqwuyrasjdf",  //邮箱授权码
  "mailserver": "smtp.qq.com",     //发送邮件服务器
  "mailserverport": 587,           //发送邮件服务器端口
  "sendwx": true,                  //是否以微信小程序方式推送短消息
  "wxcorpid": "wwc99f328ac88hasjdhf1c", //企业微信ID
  "wxcorpsecret": "alksdjfklajsdflkajsdlfkjalskdfjl", //企业微信自建应用密钥
  "wxagentid": 1000011,  //企业微信自建应用编号
  "wxuser": "HAHAHAHHAHA",  //能够接收消息的账号ID
  "checkcputemp": true,  //是否检测CPU温度
  "tempinterval": 10,   //温度检测间隔时间
  "cpufanstart":  55,   //启动风扇温度值
  "cpufanconpin": 21,   //控制风扇开关的GPIO pin脚编号
  "cputempfile": "/sys/class/thermal/thermal_zone0/temp" //保存CPU温度的文件完整路径
  
  "aeskey": "akjsdflkjasdlkfjal;skdjflkasdjflkajsdf",  //对应微信接收消息的EncodingAESKey加密密钥
  "token": "aklsdjflkajsdflk;jasdlkfjlaksdjf", //对应微信接收消息的token
  "secretword": "aslkdfjlaksdjflkajsdf;cf1e",  //中转服务器的验证口令
  "headerserver": "nginx",                    //中转服务器伪装为何种服务器信息
  "fakebody": "<html><body><h1>It works!</h1></body></html>",  //中转服务器伪装页面
  "targeturl": "https://88.88.88.88/",  //对应微信接收消息的中转服务器URL
  "bdyykey": "asdkfjlkjLKJDLKSDJF",     //百度AI语音识别 key
  "bdyysecret": "SKLDJFKLSJDFSlkajsdfkljalsdjf",  //百度AI语音识别 secret
  "cuid": "f123sadfj23234",    //百度AI语音识别ID
  "port": 443,                 //中转服务器开放端口
  "ssl": true,                 //中转服务器是否是https
  "checkurl": "/check",        //中转服务器校验URL
  "certfile": "./server.pem",  
  "keyfile": "./server.key",
  "cmdfile": "/tmp/test.txt"  //扩展指令文件路径
}
```

---

针对config.json中的cmdfile的配置文件信息说明如下：


	开灯::http://192.168.1.14/led/on
	关灯::http://192.168.1.14/led/off
	温度::http://192.168.1.14/temp
	湿度::http://192.168.1.14/humi

以::为分隔符,其中的IP地址为ESP8266连接WIFI获得的IP地址,可以自行扩展指令

---

QQ邮箱建立授权码的方法如下：

[QQ邮箱帮助](https://service.mail.qq.com/cgi-bin/help?subtype=1&id=28&no=1001256)

微信开通QQ邮箱提醒方法链接如下：

[百度经验](https://jingyan.baidu.com/article/597a064374d11d312a52434c.html)

个人开通企业微信的方法如下，只是个人使用不需要进行企业认证：

[企微学院](http://wbg.do1.com.cn/xueyuan/1681.html)

在建立好的企业微信当中可以建立一个自建应用

[企业自建应用](https://open.work.weixin.qq.com/wwopen/helpguide/detail?t=selfBuildApp)

企业应用开启接收消息

[企业应用接收消息](https://work.weixin.qq.com/api/doc#90000/90135/90237)

百度AI开放平台

[百度AI开放平台](https://ai.baidu.com/)

百度语音识别

[百度语音识别](https://ai.baidu.com/docs#/ASR-API/top)

三极管控制风扇参考

[杨仕航的博客](http://yshblog.com/blog/55)

SIM900 AT指令手册

[AT指令手册](https://www.espruino.com/datasheets/SIM900_AT.pdf)

EC20 AT指令手册

[AT指令手册](https://docs-asia.electrocomponents.com/webdocs/147d/0900766b8147dbbc.pdf)

ESP8266烧录

[烧录过程](https://www.wandianshenme.com/play/esptooolpy-flash-espeasy-firmware/)

ESP8266固件

[固件申领](https://nodemcu-build.com/)

ESP8266 httpserver

[ESP8266 httpserver](https://github.com/wangzexi/NodeMCU-HTTP-Server.git)


### **问题**

* 在使用SIM900A 作为短信接收设备时，不建议使用联通卡，除非你确信附近的联通基站还在开通2G
* 从个人的使用效果来看，极力推荐使用4G模块，因为不论是从性能上，还是稳定性以及安全角度来看，4G模块都是首选选择，其次建议购买天线。
* SIM900A的AT指令与EC20的指令有些细节点上不一样，需要修改一下代码，我将会在代码当中注释出来
* 编译好的二进制程序持续运行加入进rc.local文件，如果使用了4G模块，建议sleep 10秒以上，不然有极大概率会启动失败，怀疑是硬件本身没有初始化好设备文件导致的。
* 腾讯的语音识别在企业版免费应用上未开通,只能使用百度的,总体感觉百度的语音识别相对腾讯要稍差一些.
* ESP8266 是一个很不错的IoT开发模块,推荐大家购买

### **可扩展功能**
* 目前硬盘或主机的开关机，文件的下载, IoT的控制以及发送短信,拨打电话都已经实现。
* 目前测试通过的是QQ邮箱，其他邮箱应该是同理的，需要做测试。
* 经过验证,目前已经可以跑在jetson nano上了,不过需要定制内核并重新编译

