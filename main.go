package main

//执行方式：
//alter -to=接收人 -agent=应用id	-color=消息头部颜色	-corpid=corpid		   -corpsecret=corpsecret
//alter -to=@all  -agent=29481187 -color=FFE61A1A 	-corpid=dingd123465865 -corpsecret=zC5Jbed9S
//CorpID和CorpSecret可以在微信后台找到
import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
)

type MsgInfo struct {
	//消息属性和内容
	To, Corpid, Corpsecret, Msg, Url, Style string
	Agentid                                 int
}

var msgInfo MsgInfo

type Alter struct {
	From                   string `json:"from" xml:"from"`
	Time                   string `json:"time" xml:"time"`
	Level                  string `json:"level" xml:"level"`
	Name                   string `json:"name" xml:"name"`
	Key                    string `json:"key" xml:"key"`
	Value                  string `json:"value" xml:"value"`
	Now                    string `json:"now" xml:"now"`
	ID                     string `json:"id" xml:"id"`
	IP                     string `json:"ip" xml:"ip"`
	Color                  string `json:"color" xml:"color"`
	Age                    string `json:"age" xml:"age"`
	Status                 string `json:"status" xml:"status"`
	RecoveryTime           string `json:"recoveryTime" xml:"recoveryTime"`
	Acknowledgement        string `json:"acknowledgement" xml:"acknowledgement"`
	Acknowledgementhistory string `json:"acknowledgementhistory" xml:"acknowledgementhistory"`
}

type WechatMsg struct {
	Touser  string `json:"touser"`
	Toparty string `json:"toparty"`
	Totag   string `json:"totag"`
	Msgtype string `json:"msgtype"`
	Agentid int    `json:"agentid"`
	Text    struct {
		Content string `json:"content"`
	} `json:"text"`
	Safe int `json:"safe"`
}

func init() {
	//	log.SetFlags(log.Lshortfile | log.LstdFlags)
	flag.StringVar(&msgInfo.To, "to", "@all", "消息的接收人，可以在微信后台查看，可空。")
	flag.IntVar(&msgInfo.Agentid, "agentid", 1, "AgentID，可以在微信后台查看，不可空。")
	flag.StringVar(&msgInfo.Corpid, "corpid", "", "CorpID，可以在微信后台查看，不可空。")
	flag.StringVar(&msgInfo.Corpsecret, "corpsecret", "", "CorpSecret，可以在微信后台查看，不可空。")
	flag.StringVar(&msgInfo.Msg, "msg", `{ "from": "千思网", "time": "2016.07.28 17:00:05", "level": "Warning", "name": "这是一个千思网（qiansw.com）提供的ZABBIX微信报警插件。", "key": "icmpping", "value": "30ms", "now": "56ms", "id": "1637", "ip": "8.8.8.8", "color":"FF4A934A", "age":"3m", "recoveryTime":"2016.07.28 17:03:05", "status":"OK" }`, "Json格式的文本消息内容，不可空。")
	flag.StringVar(&msgInfo.Url, "url", "http://www.qiansw.com/golang-zabbix-alter-to-wechat.html", "消息内容点击后跳转到的URL，可空。")
	flag.StringVar(&msgInfo.Style, "style", "json", "Msg的格式，可选json和xml，推荐使用xml（支持消息中含双引号），可空。")
	flag.Parse()
	log.Println("初始化完成。")
}

func makeMsg(msg string) string {
	//	根据json或xml文本创建消息体
	log.Println("开始创建消息。")
	var alter Alter
	if msgInfo.Style == "xml" {
		log.Println("来源消息格式为XML。")
		err := xml.Unmarshal([]byte(msg), &alter)
		if err != nil {
			log.Fatal(err)
		}
	} else if msgInfo.Style == "json" {
		log.Println("来源消息格式为Json。")
		err := json.Unmarshal([]byte(msg), &alter)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		log.Println("未指定来源消息格式，默认使用Json解析。")
		err := json.Unmarshal([]byte(msg), &alter)
		if err != nil {
			log.Fatal(err)
		}
	}
	log.Printf("来源消息为：%s。\r\n", msg)

	var wechatMsg WechatMsg

	//给wechatMsg各元素赋值
	wechatMsg.Touser = msgInfo.To
	wechatMsg.Msgtype = "text"
	wechatMsg.Agentid = 1
	wechatMsg.Safe = 0
	wechatMsg.Text.Content = fmt.Sprintf("%s\n\n级别：%s\n发生：%s\n时长：%s\nIP地址：%s\n检测项：%s\n当前值：%s\n", alter.Name, alter.Level, alter.Time, alter.Age, alter.IP, alter.Key, alter.Now)

	if alter.Status == "PROBLEM" {
		//  故障处理
		wechatMsg.Text.Content = "[故障]" + wechatMsg.Text.Content
		if strings.Replace(alter.Acknowledgement, " ", "", -1) == "Yes" {
			wechatMsg.Text.Content = wechatMsg.Text.Content + "故障已经被确认，" + alter.Acknowledgementhistory
		}
		wechatMsg.Text.Content = wechatMsg.Text.Content + fmt.Sprintf("\n[%s·%s]", alter.From, alter.ID)

	} else if alter.Status == "OK" {
		//  恢复处理
		wechatMsg.Text.Content = fmt.Sprintf("[恢复]%s\n\n级别：%s\n发生：%s\n恢复：%s\n时长：%s\nIP地址：%s\n检测项：%s\n当前值：%s\n", alter.Name, alter.Level, alter.Time, alter.RecoveryTime, alter.Age, alter.IP, alter.Key, alter.Now)
		wechatMsg.Text.Content = wechatMsg.Text.Content + fmt.Sprintf("\n[%s·%s]", alter.From, alter.ID)

	} else {
		//  其他status状况处理
		wechatMsg.Text.Content = wechatMsg.Text.Content + "ZABBIX动作配置有误，请至千思网[http://www.qiansw.com]查看具体配置文档。"
		wechatMsg.Text.Content = wechatMsg.Text.Content + fmt.Sprintf("\n[%s·%s]", alter.From, alter.ID)
	}

	//	创建post给微信的Json文本
	JsonMsg, err := json.Marshal(wechatMsg)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("消息创建完成：%s\r\n", string(JsonMsg))
	return string(JsonMsg)
}

func getToken(corpid, corpsecret string) (token string) { //根据id和secret获取AccessToken

	type ResToken struct {
		Access_token string
		Errcode      int
		Errmsg       string
	}

	urlstr := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/gettoken?corpid=%s&corpsecret=%s", corpid, corpsecret)
	u, _ := url.Parse(urlstr)
	q := u.Query()
	u.RawQuery = q.Encode()
	res, err := http.Get(u.String())

	if err != nil {
		log.Fatal(err)
		return
	}
	result, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		log.Fatal(err)
		return
	}

	var m ResToken
	err1 := json.Unmarshal(result, &m)

	if err1 == nil {
		if m.Errcode == 0 {
			log.Println("AccessToken获取成功。")
			return m.Access_token
		} else {
			log.Fatal("AccessToken获取失败，", m.Errmsg)
			return
		}
		return
	} else {
		log.Fatal("Token解析失败！")
		return
	}

}
func sendMsg(token, msg string) (status bool) { //发送OA消息，,返回成功或失败
	log.Printf("需要POST的内容：%s\r\n", msg)
	body := bytes.NewBuffer([]byte(msg))
	url := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/message/send?access_token=%s", token)
	//	fmt.Println(url)
	res, err := http.Post(url, "application/json;charset=utf-8", body)
	if err != nil {
		log.Fatal(err)
		return
	}
	result, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		log.Fatal(err)
		return
	}
	log.Printf("微信接口返回消息：%s\r\n", result)

	return
}

func main() {
	sendMsg(getToken(msgInfo.Corpid, msgInfo.Corpsecret), makeMsg(msgInfo.Msg))
}
