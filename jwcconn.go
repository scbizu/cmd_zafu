package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"

	"io/ioutil"
	"net/http/cookiejar"

	"github.com/astaxie/beego"
	"github.com/fatih/color"
	"github.com/mkideal/cli"
	"github.com/scbizu/Zafu_jwcInterface/jwc_api/jwcpkg"
	"github.com/scbizu/Zafu_jwcInterface/jwc_api/models"
	"github.com/scbizu/mahonia"
)

type argT struct {
	cli.Helper
	Username string `cli:"*u" usage:"input your student number" `
	Password string `cli:"*p" usage:"input your student password" `
	Type     string `cli:"*t" usage:"input your search type:course,exam,score" `
}

//全局Cookies
var cookies []*http.Cookie

//VIEWSTATE 唯一标识
var VIEWSTATE string

var (
	username string
	password string
)

const (

	//模拟登陆第一个入口地址
	loginURLGate0 string = "http://210.33.60.8:8080/"
	//模拟登陆第一个入口验证码地址
	vrcodeURLGate0 string = "http://210.33.60.8:8080/CheckCode.aspx"
	//首页地址
	loggedURL string = "http://210.33.60.8:8080/xs_main.aspx?xh=201305070123"
	//默认登录页
	defaultURL string = "http://210.33.60.8:8080/default2.aspx"
	//课程表
	courseURL string = "http://210.33.60.8:8080/xskbcx.aspx?xh="

	examURL string = "http://210.33.60.8:8080/xskscx.aspx?xh="
	//查成绩
	scoreURL string = "http://210.33.60.8/xscjcx.aspx?xh="
)

func checkError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

/**
* 获取这两个不知道干什么的值
 */
func getsp(url string) map[string]string {
	view, err := http.Get(url)
	checkError(err)
	//去拿__VIEWSTATE
	body, err := ioutil.ReadAll(view.Body)
	checkError(err)
	regular := `<input.type="hidden".name="__VIEWSTATE".value="(.*)" />`
	pattern := regexp.MustCompile(regular)
	VIEWSTATE := pattern.FindAllStringSubmatch(string(body), -1)
	//拿__VIEWSTATEGENERATOR
	retor := `<input.type="hidden".name="__VIEWSTATEGENERATOR".value="(.*)" />`
	patterntor := regexp.MustCompile(retor)
	VIEWSTATEGENERATOR := patterntor.FindAllStringSubmatch(string(body), -1)
	res := make(map[string]string)
	res["VIEWSTATE"] = VIEWSTATE[0][1]
	res["VIEWSTATEGENERATOR"] = VIEWSTATEGENERATOR[0][1]
	return res
}

/**
*模拟post表单
 */
func post(Rurl string, c *http.Client, username string, password string, verifyCode string, VIEWSTATE string, VIEWSTATEGENERATOR string, tempCookies []*http.Cookie) []*http.Cookie {
	postValue := url.Values{}
	cd := mahonia.NewEncoder("gb2312")
	rb := cd.ConvertString("学生")
	//准备POST的数据
	postValue.Add("txtUserName", username)
	postValue.Add("TextBox2", password)
	postValue.Add("txtSecretCode", verifyCode)
	postValue.Add("__VIEWSTATE", VIEWSTATE)
	postValue.Add("__VIEWSTATEGENERATOR", VIEWSTATEGENERATOR)
	postValue.Add("Button1", "")
	postValue.Add("lbLanguage", "")
	postValue.Add("hidPdrs", "")
	postValue.Add("hidsc", "")
	postValue.Add("RadioButtonList1", rb)
	//开始POST   这次POST到登陆界面   带上第一次请求的cookie 和 验证码  和 一些必要的数据
	postURL, _ := url.Parse(Rurl)
	Jar, _ := cookiejar.New(nil)
	Jar.SetCookies(postURL, tempCookies)
	c.Jar = Jar
	resp, _ := c.PostForm(Rurl, postValue)
	Scookies := resp.Cookies()
	return Scookies
}

//Testpage 测试结果
func Testpage(c *http.Client) string {
	//拿到这个登录成功的cookie后  再带着这个cookie 再伪造一次请求去我们想要的URL
	req, err := http.NewRequest("GET", loggedURL, nil)
	checkError(err)
	for _, v := range cookies {
		req.AddCookie(v)
	}
	finalRes, err := c.Do(req)
	checkError(err)
	allData, err := ioutil.ReadAll(finalRes.Body)
	checkError(err)
	finalRes.Body.Close()
	return string(allData)
}

//获取学生姓名
func getStuName(c *http.Client) string {
	req, err := http.NewRequest("GET", loggedURL, nil)
	checkError(err)
	finalRes, err := c.Do(req)
	checkError(err)
	allData, err := ioutil.ReadAll(finalRes.Body)
	checkError(err)
	defer finalRes.Body.Close()
	cd := mahonia.NewEncoder("gb2312")
	rb := cd.ConvertString("<span.id=\"xhxm\">(.*)同学</span>")
	//Regexp
	regular := rb
	pattern := regexp.MustCompile(regular)
	stuName := pattern.FindAllStringSubmatch(string(allData), -1)
	return stuName[0][1]
}

//Get Course info.
func getCourseData(c *http.Client) string {
	req, err := http.NewRequest("GET", courseURL, nil)
	//NICE
	req.Header.Set("Referer", courseURL)
	checkError(err)
	finalRes, err := c.Do(req)
	checkError(err)
	allData, err := ioutil.ReadAll(finalRes.Body)
	checkError(err)
	finalRes.Body.Close()
	return string(allData)
}

//GetExaminfo ..
func GetExaminfo(c *http.Client) string {
	ExamURL := examURL + username
	req, err := http.NewRequest("GET", ExamURL, nil)
	//NICE
	req.Header.Set("Referer", ExamURL)
	checkError(err)
	finalRes, err := c.Do(req)
	checkError(err)
	allData, err := ioutil.ReadAll(finalRes.Body)
	checkError(err)
	finalRes.Body.Close()
	return string(allData)
}

//GetScoreinfo ..
func GetScoreinfo(c *http.Client) (string, error) {
	ScoreURL := scoreURL + username
	beego.Debug(ScoreURL)
	req, err := http.NewRequest("GET", ScoreURL, nil)
	req.Header.Set("Referer", ScoreURL)
	if err != nil {
		return "", err
	}
	finalRes, err := c.Do(req)
	if err != nil {
		return "", err
	}
	allData, err := ioutil.ReadAll(finalRes.Body)
	if err != nil {
		return "", err
	}
	finalRes.Body.Close()
	return string(allData), nil
}

func getscoreVs(str string) string {
	//	beego.Debug(str)
	regular := `<input.type="hidden".name="__VIEWSTATE".value="(.*)" />`
	pattern := regexp.MustCompile(regular)
	res := pattern.FindAllStringSubmatch(str, -1)
	return res[0][1]
}

func getscoreVg(str string) string {
	regular := `<input.type="hidden".name="__VIEWSTATEGENERATOR".value="(.*)" />`
	pattern := regexp.MustCompile(regular)
	res := pattern.FindAllStringSubmatch(str, -1)
	return res[0][1]
}

func findOutScore(client *http.Client, Vs string, Vg string, xn string, xq string, btnxq string) string {
	ScoreURL := scoreURL + username
	getScore := url.Values{}
	cd := mahonia.NewEncoder("gb2312")
	getScore.Add("__VIEWSTATE", Vs)
	getScore.Add("__VIEWSTATEGENERATOR", Vg)
	getScore.Add("ddl_kcxz", "")
	getScore.Add("btn_zcj", cd.ConvertString("历年成绩"))
	req, err := http.NewRequest("POST", ScoreURL, bytes.NewBufferString(getScore.Encode()))
	if err != nil {
		panic(err)
	}

	req.Header.Add("Referer", ScoreURL)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Content-Length", strconv.Itoa(len(getScore.Encode())))
	Res, err := client.Do(req)
	if err != nil {

		panic(err)
	}

	data, _ := ioutil.ReadAll(Res.Body)
	defer Res.Body.Close()
	return string(data)
}

//MAIN

func main() {

	cli.Run(new(argT), func(ctx *cli.Context) error {
		argv := ctx.Argv().(*argT)
		username = argv.Username
		password = argv.Password

		viewRes := getsp(loginURLGate0)
		VIEWSTATE := viewRes["VIEWSTATE"]
		VIEWSTATEGENERATOR := viewRes["VIEWSTATEGENERATOR"]

		//获取登陆界面的cookie
		c := &http.Client{}
		req, err := http.NewRequest("GET", loginURLGate0, nil)
		if err != nil {
			return err
		}
		res, err := c.Do(req)
		if err != nil {
			return err
		}
		var tempCookies = res.Cookies()
		//第二次 带着登陆界面的cookie去验证码页面拿验证码
		req.URL, _ = url.Parse(vrcodeURLGate0)
		for _, v := range res.Cookies() {
			req.AddCookie(v)
		}
		// 获取验证码
		var verifyCode string
		for {
			//用刚才生成的cookie去爬 验证码   否则会504!!!!!

			res, err = c.Do(req)
			if err != nil {
				return err
			}
			file, err := os.Create("./code.gif")
			if err != nil {
				return err
			}
			io.Copy(file, res.Body)

			fmt.Println("请查看code.gif， 然后输入验证码， 看不清输入0重新获取验证码")
			fmt.Scanf("%s", &verifyCode)
			if verifyCode != "0" {
				break
			}
			res.Body.Close()
		}

		post(defaultURL, c, username, password, verifyCode, VIEWSTATE, VIEWSTATEGENERATOR, tempCookies)
		switch argv.Type {
		case "course":
			// TODO:

			break
		case "exam":
			exam := GetExaminfo(c)
			examInfo := jwcpkg.FetchExam(exam)
			for k, v := range examInfo {
				color.Black("NUM: " + k + " Class: " + v.Class + " Deadline: " + v.Deadline)
			}
			break
		case "score":

			info, err := GetScoreinfo(c)
			if err != nil {
				return err
			}
			vs := getscoreVs(info)
			vg := getscoreVg(info)
			data := models.FindOutScore(c, vs, vg, "", "", "")
			scoreInfo := jwcpkg.FetchScoreTD(data)
			for k, v := range scoreInfo {
				color.Black("NUM: " + k + "课程:" + v.ClassName + "成绩:" + v.Score + "GPA:" + v.GPA + "绩点:" + v.Credit + "开课学院:" + v.Academy)
			}
			break
		default:
			color.Red("Nothing")
			break
		}
		return nil
	}, "CLI For zafuJwc")

}
