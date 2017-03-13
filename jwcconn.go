package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"

	"io/ioutil"
	"net/http/cookiejar"

	"github.com/PuerkitoBio/goquery"
	"github.com/fatih/color"
	"github.com/mkideal/cli"
	"github.com/scbizu/Zafu_jwcInterface/jwc_api/jwcpkg"
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
	loginURLGate0 string = "http://210.33.60.5/"
	//模拟登陆第一个入口验证码地址
	vrcodeURLGate0 string = "http://210.33.60.5/CheckCode.aspx"
	//首页地址
	// loggedURL string = "http://210.33.60.8:8080/xs_main.aspx?xh="
	//默认登录页
	defaultURL string = "http://210.33.60.5/default2.aspx"
	//课程表
	courseURL string = "http://210.33.60.5/xskbcx.aspx?xh="

	examURL string = "http://210.33.60.5/xskscx.aspx?xh="
	//查成绩
	scoreURL string = "http://210.33.60.5/xscjcx.aspx?xh="
)

/**
* magic string
 */
func getsp(url string) (map[string]string, error) {
	view, err := http.Get(url)
	if err != nil {
		return nil, errors.New("发送请求失败(获取SP)")
	}
	//去拿__VIEWSTATE
	body, err := ioutil.ReadAll(view.Body)
	if err != nil {
		return nil, errors.New("获取body体失败啦～(获取SP)")
	}
	regular := `<input.type="hidden".name="__VIEWSTATE".value="(.*)" />`
	pattern := regexp.MustCompile(regular)
	VIEWSTATE := pattern.FindAllStringSubmatch(string(body), -1)
	//拿__VIEWSTATEGENERATOR
	retor := `<input.type="hidden".name="__VIEWSTATEGENERATOR".value="(.*)" />`
	patterntor := regexp.MustCompile(retor)
	VIEWSTATEGENERATOR := patterntor.FindAllStringSubmatch(string(body), -1)
	res := make(map[string]string)
	if len(VIEWSTATE) > 0 {
		res["VIEWSTATE"] = VIEWSTATE[0][1]
	} else {
		res["VIEWSTATE"] = ""
	}
	if len(VIEWSTATEGENERATOR) > 0 {
		res["VIEWSTATEGENERATOR"] = VIEWSTATEGENERATOR[0][1]
	} else {
		res["VIEWSTATEGENERATOR"] = ""
	}

	return res, nil
}

/**
*模拟post表单
 */
func post(Rurl string, c *http.Client, username string, password string, verifyCode string, VIEWSTATE string, VIEWSTATEGENERATOR string, tempCookies []*http.Cookie) ([]*http.Cookie, error) {
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
	resp, err := c.PostForm(Rurl, postValue)
	if err != nil {
		return nil, err
	}
	Scookies := resp.Cookies()
	return Scookies, nil
}

//Get Course info.
func getCourseData(c *http.Client) (*goquery.Document, error) {
	CourseURL := courseURL + username
	req, err := http.NewRequest("GET", CourseURL, nil)
	if err != nil {
		return nil, errors.New("发送Request请求失败~")
	}
	//refer
	req.Header.Set("Referer", courseURL)
	finalRes, err := c.Do(req)
	if err != nil {
		return nil, errors.New("请检查输入数据是否正确(用户名,密码,验证码)")
	}

	res, err := ioutil.ReadAll(finalRes.Body)
	if err != nil {
		return nil, err
	}
	defer finalRes.Body.Close()

	r := strings.NewReader(string(res))
	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		return nil, errors.New("Goquery死掉了...")
	}

	return doc, nil
}

func resolveCourseData(doc *goquery.Document) map[string][]string {
	allclass := make(map[string][]string, 100)
	doc.Find(".blacktab tbody tr").Each(func(trIndex int, trData *goquery.Selection) {
		if trIndex == 1 || trIndex == 2 || trIndex == 4 || trIndex == 5 || trIndex == 7 || trIndex == 9 || trIndex == 11 {
			if trIndex == 2 || trIndex == 7 || trIndex == 11 {
				trData.Find("td").Each(func(tdIndex int, tdData *goquery.Selection) {
					if len(tdData.Text()) != 2 {
						switch tdIndex {
						case 2:
							allclass["Monday"] = append(allclass["Monday"], tdData.Text())
						case 3:
							allclass["Tuesday"] = append(allclass["Tuesday"], tdData.Text())
						case 4:
							allclass["Wednesday"] = append(allclass["Wednesday"], tdData.Text())
						case 5:
							allclass["Thursday"] = append(allclass["Thursday"], tdData.Text())
						case 6:
							allclass["Friday"] = append(allclass["Friday"], tdData.Text())
						case 7:
							allclass["Saturday"] = append(allclass["Saturday"], tdData.Text())
						case 8:
							allclass["Sunday"] = append(allclass["Sunday"], tdData.Text())
						}
					}
				})
			} else {
				trData.Find("td").Each(func(tdIndex int, tdData *goquery.Selection) {
					if len(tdData.Text()) != 2 {
						switch tdIndex {
						case 1:
							allclass["Monday"] = append(allclass["Monday"], tdData.Text())
						case 2:
							allclass["Tuesday"] = append(allclass["Tuesday"], tdData.Text())
						case 3:
							allclass["Wednesday"] = append(allclass["Wednesday"], tdData.Text())
						case 4:
							allclass["Thursday"] = append(allclass["Thursday"], tdData.Text())
						case 5:
							allclass["Friday"] = append(allclass["Friday"], tdData.Text())
						case 6:
							allclass["Saturday"] = append(allclass["Saturday"], tdData.Text())
						case 7:
							allclass["Sunday"] = append(allclass["Sunday"], tdData.Text())
						}
					}
				})
			}
		}
	})
	return allclass
}

//GetExaminfo ..
func getExaminfo(c *http.Client) (string, error) {
	ExamURL := examURL + username
	req, err := http.NewRequest("GET", ExamURL, nil)
	req.Header.Set("Referer", ExamURL)
	if err != nil {
		return "", errors.New("发送请求失败了~")
	}
	finalRes, err := c.Do(req)
	if err != nil {
		return "", errors.New("获取失败,检查验证码,学号,密码输入是否合法")
	}
	allData, err := ioutil.ReadAll(finalRes.Body)
	if err != nil {
		return "", errors.New("读取body失败")
	}
	defer finalRes.Body.Close()
	return string(allData), nil
}

//GetScoreinfo ..
func getScoreinfo(c *http.Client) (string, error) {
	ScoreURL := scoreURL + username
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
	defer finalRes.Body.Close()
	return string(allData), nil
}

func getscoreVs(str string) string {
	//	beego.Debug(str)
	regular := `<input.type="hidden".name="__VIEWSTATE".value="(.*)" />`
	pattern := regexp.MustCompile(regular)
	res := pattern.FindAllStringSubmatch(str, -1)
	if len(res) == 0 {
		return ""
	}
	return res[0][1]
}

func getscoreVg(str string) string {
	regular := `<input.type="hidden".name="__VIEWSTATEGENERATOR".value="(.*)" />`
	pattern := regexp.MustCompile(regular)
	res := pattern.FindAllStringSubmatch(str, -1)
	if len(res) == 0 {
		return ""
	}
	return res[0][1]
}

func findOutScore(client *http.Client, Vs string, Vg string, xn string, xq string, btnxq string) (string, error) {
	ScoreURL := scoreURL + username
	getScore := url.Values{}
	cd := mahonia.NewEncoder("gb2312")
	getScore.Add("__VIEWSTATE", Vs)
	getScore.Add("__VIEWSTATEGENERATOR", Vg)
	getScore.Add("ddl_kcxz", "")
	getScore.Add("btn_zcj", cd.ConvertString("历年成绩"))
	req, err := http.NewRequest("POST", ScoreURL, bytes.NewBufferString(getScore.Encode()))
	if err != nil {
		return "", errors.New("发送POST请求失败~")
	}

	req.Header.Add("Referer", ScoreURL)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Content-Length", strconv.Itoa(len(getScore.Encode())))
	Res, err := client.Do(req)
	if err != nil {
		return "", errors.New("输入不正确")
	}

	data, _ := ioutil.ReadAll(Res.Body)
	defer Res.Body.Close()
	return string(data), nil
}

//MAIN

func main() {

	fn := func(ctx *cli.Context) error {
		argv := ctx.Argv().(*argT)
		username = argv.Username
		password = argv.Password

		viewRes, err := getsp(loginURLGate0)
		if err != nil {
			color.Red(err.Error())
		}
		VIEWSTATE := viewRes["VIEWSTATE"]
		VIEWSTATEGENERATOR := viewRes["VIEWSTATEGENERATOR"]

		//获取登陆界面的cookie
		c := &http.Client{}
		req, err := http.NewRequest("GET", loginURLGate0, nil)
		if err != nil {
			color.Red("发送Request失败了(获取Cookies)")
			os.Exit(1)
		}
		res, err := c.Do(req)
		if err != nil {
			color.Red("没有Response返回(获取Cookies)")
			os.Exit(1)
		}
		var tempCookies = res.Cookies()
		//第二次 带着登陆界面的cookie去验证码页面拿验证码
		req.URL, _ = url.Parse(vrcodeURLGate0)
		for _, v := range res.Cookies() {
			req.AddCookie(v)
		}
		color.Green("获取主页Cookies成功....")
		color.Green("开始插入Cookies...")
		// 获取验证码
		var verifyCode string
		for {
			res, err = c.Do(req)
			if err != nil {
				color.Red("发送Request失败了(获取验证码)")
				os.Exit(1)
			}
			file, er := os.Create("./code.gif")
			if er != nil {
				color.Red("创建验证码图片遇到了错误...请允许写文件的权限o~o")
				os.Exit(1)
			}
			io.Copy(file, res.Body)

			fmt.Println("请查看code.gif， 然后输入验证码， 看不清输入0重新获取验证码")
			fmt.Scanf("%s", &verifyCode)
			if verifyCode != "0" {
				color.Green("验证码输入成功,正在请求教务处...")
				break
			}
			defer res.Body.Close()
		}
		//POST
		_, err = post(defaultURL, c, username, password, verifyCode, VIEWSTATE, VIEWSTATEGENERATOR, tempCookies)
		if err != nil {
			color.Red("POST失败~")
			os.Exit(1)
		}
		//OP
		switch argv.Type {
		case "course":
			course, err := getCourseData(c)
			if err != nil {
				color.Red(err.Error())
				os.Exit(1)
			}
			color.Green("正在生成课程.....")
			courseInfo := resolveCourseData(course)
			for day, class := range courseInfo {
				color.Black(day + ":")
				fmt.Println()
				for _, v := range class {
					cd := mahonia.NewDecoder("GBK")
					if err != nil {
						color.Red("字符转换失败了...")
						os.Exit(1)
					}
					color.Black(cd.ConvertString(v) + "\n")
					fmt.Println()
				}
			}
			break
		case "exam":
			color.Green("正在生成考试信息...")
			exam, err := getExaminfo(c)
			if err != nil {
				color.Red(err.Error())
				os.Exit(1)
			}
			examInfo := jwcpkg.FetchExam(exam)
			for k, v := range examInfo {
				color.Black(k + "		" + " Class: " + v.Class + "		" + " Deadline: " + "		" + v.Deadline)
			}
			break
		case "score":
			color.Green("正在生成成绩信息,不要紧张,深呼吸~")
			info, err := getScoreinfo(c)
			if err != nil {
				color.Red(err.Error())
				os.Exit(1)
			}
			vs := getscoreVs(info)
			vg := getscoreVg(info)
			data, err := findOutScore(c, vs, vg, "", "", "")
			if err != nil {
				color.Red(err.Error())
			}
			scoreInfo := jwcpkg.FetchScoreTD(data)
			for _, v := range scoreInfo {
				color.Black("课程:" + v.ClassName + "		" + "成绩:" + v.Score + "		" + "绩点:" + v.GPA + "		" + "学分:" + v.Credit + "		")
				fmt.Println()
			}
			break
		default:
			color.Red("Nothing")
			break
		}
		return nil
	}
	cli.Run(new(argT), fn, "CLI For zafuJwc")

}
