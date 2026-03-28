package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	_ "modernc.org/sqlite"
)

var db *sql.DB

// 统一跨域处理
func setCORS(w http.ResponseWriter, r *http.Request) bool {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	// 处理预检请求
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusNoContent)
		return true
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	return false
}

func main() {
	// 初始化数据库
	var err error
	db, err = sql.Open("sqlite", "./qymail.db")
	if err != nil {
		fmt.Println("❌ 数据库打开失败:", err)
		return
	}
	defer db.Close()

	// 测试数据库连接
	err = db.Ping()
	if err != nil {
		fmt.Println("❌ 数据库连接失败:", err)
		return
	}

	createTables()
	fmt.Println("✅ 数据库初始化成功！")
	fmt.Println("✅ QYmail 联系人版启动成功！http://127.0.0.1:8080")

	// 原有功能路由
	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/api/register", registerHandler)
	http.HandleFunc("/api/login", loginHandler)
	http.HandleFunc("/api/emails", emailsHandler)
	http.HandleFunc("/api/send", sendHandler)
	http.HandleFunc("/api/detail", detailHandler)
	http.HandleFunc("/api/delete", deleteHandler)

	// 🔥 新增：联系人功能路由
	http.HandleFunc("/api/contact/add", contactAddHandler)    // 新增联系人
	http.HandleFunc("/api/contact/list", contactListHandler)  // 获取联系人列表
	http.HandleFunc("/api/contact/edit", contactEditHandler)  // 编辑联系人
	http.HandleFunc("/api/contact/delete", contactDeleteHandler) // 删除联系人

	// 启动服务
	http.ListenAndServe("127.0.0.1:8080", nil)
}

// 首页
func homeHandler(w http.ResponseWriter, r *http.Request) {
	if setCORS(w, r) { return }
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, `<h1 style="color:#e63946;">QYmail 联系人版已启动</h1>`)
}

// 注册接口
func registerHandler(w http.ResponseWriter, r *http.Request) {
	if setCORS(w, r) { return }
	if r.Method != "POST" {
		fmt.Fprint(w, `{"success":false,"message":"仅支持POST请求"}`)
		return
	}

	username := strings.TrimSpace(r.FormValue("username"))
	password := strings.TrimSpace(r.FormValue("password"))
	userEmail := username + "@qy.com"

	if username == "" || password == "" {
		fmt.Fprint(w, `{"success":false,"message":"用户名/密码不能为空"}`)
		return
	}

	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM users WHERE username=?", username).Scan(&count)
	if err != nil {
		fmt.Fprintf(w, `{"success":false,"message":"系统错误：%s"}`, err.Error())
		return
	}
	if count > 0 {
		fmt.Fprint(w, `{"success":false,"message":"用户名已被注册"}`)
		return
	}

	_, err = db.Exec("INSERT INTO users (username,password,email) VALUES (?,?,?)", username, password, userEmail)
	if err != nil {
		fmt.Fprintf(w, `{"success":false,"message":"注册失败：%s"}`, err.Error())
		return
	}

	_, err = db.Exec("INSERT INTO emails (to_email,from_email,subject,content) VALUES (?,?,?,?)",
		userEmail, "admin@qy.com", "欢迎使用QYmail", "你的专属邮箱已开通！现在你可以添加联系人，一键发邮件啦。")
	if err != nil {
		fmt.Println("⚠️ 欢迎邮件发送失败：", err)
	}

	fmt.Printf("✅ 新用户注册成功：%s\n", userEmail)
	fmt.Fprintf(w, `{"success":true,"email":"%s","message":"注册成功"}`, userEmail)
}

// 登录接口
func loginHandler(w http.ResponseWriter, r *http.Request) {
	if setCORS(w, r) { return }
	if r.Method != "POST" {
		fmt.Fprint(w, `{"success":false,"message":"仅支持POST请求"}`)
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")

	var dbPwd, userEmail string
	err := db.QueryRow("SELECT password,email FROM users WHERE username=?", username).Scan(&dbPwd, &userEmail)
	if err != nil {
		fmt.Fprint(w, `{"success":false,"message":"用户不存在"}`)
		return
	}
	if dbPwd != password {
		fmt.Fprint(w, `{"success":false,"message":"密码错误"}`)
		return
	}

	fmt.Printf("✅ 用户登录成功：%s\n", userEmail)
	fmt.Fprintf(w, `{"success":true,"email":"%s","message":"登录成功"}`, userEmail)
}

// 邮件列表接口（用户隔离）
func emailsHandler(w http.ResponseWriter, r *http.Request) {
	if setCORS(w, r) { return }

	userEmail := r.URL.Query().Get("user_email")
	if userEmail == "" {
		fmt.Fprint(w, `{"success":false,"message":"用户身份校验失败，请重新登录"}`)
		return
	}

	rows, err := db.Query("SELECT id,from_email,subject,content FROM emails WHERE to_email=? ORDER BY id DESC", userEmail)
	if err != nil {
		fmt.Fprintf(w, `{"success":false,"message":"查询失败：%s"}`, err.Error())
		return
	}
	defer rows.Close()

	var emails []string
	for rows.Next() {
		var id int
		var from, subject, content string
		err := rows.Scan(&id, &from, &subject, &content)
		if err != nil {
			continue
		}
		preview := content
		if len(preview) > 50 {
			preview = preview[:50] + "..."
		}
		emails = append(emails, fmt.Sprintf(`{"id":%d,"from":"%s","subject":"%s","preview":"%s","content":"%s"}`, id, from, subject, preview, content))
	}

	fmt.Fprint(w, `{"success":true,"emails":[`+strings.Join(emails, ",")+`]}`)
}

// 发邮件接口
func sendHandler(w http.ResponseWriter, r *http.Request) {
	if setCORS(w, r) { return }
	fmt.Println("\n📤 收到发邮件请求")

	if r.Method != "POST" {
		fmt.Println("❌ 错误：不是POST请求")
		fmt.Fprint(w, `{"success":false,"message":"仅支持POST请求"}`)
		return
	}

	err := r.ParseForm()
	if err != nil {
		fmt.Println("❌ 参数解析失败:", err)
		fmt.Fprintf(w, `{"success":false,"message":"参数解析失败：%s"}`, err.Error())
		return
	}

	fromEmail := strings.TrimSpace(r.FormValue("from_email"))
	toEmail := strings.TrimSpace(r.FormValue("to_email"))
	subject := strings.TrimSpace(r.FormValue("subject"))
	content := strings.TrimSpace(r.FormValue("content"))

	if fromEmail == "" || toEmail == "" || subject == "" {
		fmt.Println("❌ 错误：必填参数为空")
		fmt.Fprint(w, `{"success":false,"message":"发件人/收件人/主题不能为空"}`)
		return
	}

	var userCount int
	err = db.QueryRow("SELECT COUNT(*) FROM users WHERE email=?", fromEmail).Scan(&userCount)
	if err != nil || userCount == 0 {
		fmt.Println("❌ 错误：发件人身份非法")
		fmt.Fprint(w, `{"success":false,"message":"发件人身份非法，请重新登录"}`)
		return
	}

	var toCount int
	err = db.QueryRow("SELECT COUNT(*) FROM users WHERE email=?", toEmail).Scan(&toCount)
	if err != nil || toCount == 0 {
		fmt.Println("❌ 错误：收件人不存在")
		fmt.Fprint(w, `{"success":false,"message":"收件人不存在，请确认邮箱地址正确"}`)
		return
	}

	_, err = db.Exec("INSERT INTO emails (to_email,from_email,subject,content) VALUES (?,?,?,?)", toEmail, fromEmail, subject, content)
	if err != nil {
		fmt.Println("❌ 数据库写入失败:", err)
		fmt.Fprintf(w, `{"success":false,"message":"发送失败：%s"}`, err.Error())
		return
	}

	fmt.Println("✅ 邮件发送成功！")
	fmt.Fprint(w, `{"success":true,"message":"邮件发送成功！"}`)
}

// 邮件详情接口
func detailHandler(w http.ResponseWriter, r *http.Request) {
	if setCORS(w, r) { return }

	emailID := r.URL.Query().Get("id")
	userEmail := r.URL.Query().Get("user_email")

	if emailID == "" || userEmail == "" {
		fmt.Fprint(w, `{"success":false,"message":"参数缺失或身份校验失败"}`)
		return
	}

	id, err := strconv.Atoi(emailID)
	if err != nil {
		fmt.Fprint(w, `{"success":false,"message":"邮件ID无效"}`)
		return
	}

	var from, subject, content string
	err = db.QueryRow("SELECT from_email,subject,content FROM emails WHERE id=? AND to_email=?", id, userEmail).Scan(&from, &subject, &content)
	if err != nil {
		fmt.Fprint(w, `{"success":false,"message":"邮件不存在或无权限查看"}`)
		return
	}

	fmt.Fprintf(w, `{"success":true,"from":"%s","subject":"%s","content":"%s"}`, from, subject, content)
}

// 删除邮件接口
func deleteHandler(w http.ResponseWriter, r *http.Request) {
	if setCORS(w, r) { return }
	if r.Method != "POST" {
		fmt.Fprint(w, `{"success":false,"message":"仅支持POST请求"}`)
		return
	}

	emailID := r.FormValue("id")
	userEmail := r.FormValue("user_email")

	if emailID == "" || userEmail == "" {
		fmt.Fprint(w, `{"success":false,"message":"参数缺失或身份校验失败"}`)
		return
	}

	id, err := strconv.Atoi(emailID)
	if err != nil {
		fmt.Fprint(w, `{"success":false,"message":"邮件ID无效"}`)
		return
	}

	result, err := db.Exec("DELETE FROM emails WHERE id=? AND to_email=?", id, userEmail)
	if err != nil {
		fmt.Fprintf(w, `{"success":false,"message":"删除失败：%s"}`, err.Error())
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		fmt.Fprint(w, `{"success":false,"message":"邮件不存在或无权限删除"}`)
		return
	}

	fmt.Fprint(w, `{"success":true,"message":"删除成功！"}`)
}

// ==================== 🔥 联系人功能核心接口 ====================
// 新增联系人
func contactAddHandler(w http.ResponseWriter, r *http.Request) {
	if setCORS(w, r) { return }
	fmt.Println("\n👤 收到新增联系人请求")

	if r.Method != "POST" {
		fmt.Fprint(w, `{"success":false,"message":"仅支持POST请求"}`)
		return
	}

	err := r.ParseForm()
	if err != nil {
		fmt.Fprintf(w, `{"success":false,"message":"参数解析失败：%s"}`, err.Error())
		return
	}

	// 获取参数
	userEmail := strings.TrimSpace(r.FormValue("user_email"))
	contactName := strings.TrimSpace(r.FormValue("contact_name"))
	contactEmail := strings.TrimSpace(r.FormValue("contact_email"))
	contactNote := strings.TrimSpace(r.FormValue("contact_note"))

	// 校验
	if userEmail == "" || contactName == "" || contactEmail == "" {
		fmt.Println("❌ 错误：必填参数为空")
		fmt.Fprint(w, `{"success":false,"message":"所属用户、联系人姓名、邮箱不能为空"}`)
		return
	}

	// 校验：同一个用户不能添加重复的联系人邮箱
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM contacts WHERE user_email=? AND contact_email=?", userEmail, contactEmail).Scan(&count)
	if err != nil {
		fmt.Fprintf(w, `{"success":false,"message":"系统错误：%s"}`, err.Error())
		return
	}
	if count > 0 {
		fmt.Println("❌ 错误：联系人已存在")
		fmt.Fprint(w, `{"success":false,"message":"该联系人邮箱已存在"}`)
		return
	}

	// 写入数据库
	_, err = db.Exec("INSERT INTO contacts (user_email,contact_name,contact_email,contact_note) VALUES (?,?,?,?)",
		userEmail, contactName, contactEmail, contactNote)
	if err != nil {
		fmt.Println("❌ 新增联系人失败:", err)
		fmt.Fprintf(w, `{"success":false,"message":"新增失败：%s"}`, err.Error())
		return
	}

	fmt.Println("✅ 联系人新增成功！")
	fmt.Fprint(w, `{"success":true,"message":"联系人添加成功！"}`)
}

// 获取联系人列表（仅当前用户的）
func contactListHandler(w http.ResponseWriter, r *http.Request) {
	if setCORS(w, r) { return }

	userEmail := r.URL.Query().Get("user_email")
	if userEmail == "" {
		fmt.Fprint(w, `{"success":false,"message":"用户身份校验失败"}`)
		return
	}

	fmt.Printf("👤 用户 %s 请求联系人列表\n", userEmail)

	// 仅查询当前用户的联系人
	rows, err := db.Query("SELECT id,contact_name,contact_email,contact_note FROM contacts WHERE user_email=? ORDER BY id DESC", userEmail)
	if err != nil {
		fmt.Fprintf(w, `{"success":false,"message":"查询失败：%s"}`, err.Error())
		return
	}
	defer rows.Close()

	var contacts []string
	for rows.Next() {
		var id int
		var name, email, note string
		err := rows.Scan(&id, &name, &email, &note)
		if err != nil {
			continue
		}
		contacts = append(contacts, fmt.Sprintf(`{"id":%d,"name":"%s","email":"%s","note":"%s"}`, id, name, email, note))
	}

	fmt.Printf("✅ 返回 %d 个联系人\n", len(contacts))
	fmt.Fprint(w, `{"success":true,"contacts":[`+strings.Join(contacts, ",")+`]}`)
}

// 编辑联系人
func contactEditHandler(w http.ResponseWriter, r *http.Request) {
	if setCORS(w, r) { return }
	fmt.Println("\n👤 收到编辑联系人请求")

	if r.Method != "POST" {
		fmt.Fprint(w, `{"success":false,"message":"仅支持POST请求"}`)
		return
	}

	err := r.ParseForm()
	if err != nil {
		fmt.Fprintf(w, `{"success":false,"message":"参数解析失败：%s"}`, err.Error())
		return
	}

	contactID := r.FormValue("id")
	userEmail := strings.TrimSpace(r.FormValue("user_email"))
	contactName := strings.TrimSpace(r.FormValue("contact_name"))
	contactEmail := strings.TrimSpace(r.FormValue("contact_email"))
	contactNote := strings.TrimSpace(r.FormValue("contact_note"))

	if contactID == "" || userEmail == "" || contactName == "" || contactEmail == "" {
		fmt.Println("❌ 错误：必填参数为空")
		fmt.Fprint(w, `{"success":false,"message":"参数缺失"}`)
		return
	}

	id, err := strconv.Atoi(contactID)
	if err != nil {
		fmt.Fprint(w, `{"success":false,"message":"联系人ID无效"}`)
		return
	}

	// 校验归属权
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM contacts WHERE id=? AND user_email=?", id, userEmail).Scan(&count)
	if err != nil || count == 0 {
		fmt.Println("❌ 错误：无权限编辑该联系人")
		fmt.Fprint(w, `{"success":false,"message":"联系人不存在或无权限编辑"}`)
		return
	}

	// 执行更新
	_, err = db.Exec("UPDATE contacts SET contact_name=?, contact_email=?, contact_note=? WHERE id=? AND user_email=?",
		contactName, contactEmail, contactNote, id, userEmail)
	if err != nil {
		fmt.Println("❌ 编辑联系人失败:", err)
		fmt.Fprintf(w, `{"success":false,"message":"编辑失败：%s"}`, err.Error())
		return
	}

	fmt.Println("✅ 联系人编辑成功！")
	fmt.Fprint(w, `{"success":true,"message":"联系人编辑成功！"}`)
}

// 删除联系人
func contactDeleteHandler(w http.ResponseWriter, r *http.Request) {
	if setCORS(w, r) { return }
	fmt.Println("\n👤 收到删除联系人请求")

	if r.Method != "POST" {
		fmt.Fprint(w, `{"success":false,"message":"仅支持POST请求"}`)
		return
	}

	err := r.ParseForm()
	if err != nil {
		fmt.Fprintf(w, `{"success":false,"message":"参数解析失败：%s"}`, err.Error())
		return
	}

	contactID := r.FormValue("id")
	userEmail := strings.TrimSpace(r.FormValue("user_email"))

	if contactID == "" || userEmail == "" {
		fmt.Println("❌ 错误：必填参数为空")
		fmt.Fprint(w, `{"success":false,"message":"参数缺失"}`)
		return
	}

	id, err := strconv.Atoi(contactID)
	if err != nil {
		fmt.Fprint(w, `{"success":false,"message":"联系人ID无效"}`)
		return
	}

	// 校验归属权
	result, err := db.Exec("DELETE FROM contacts WHERE id=? AND user_email=?", id, userEmail)
	if err != nil {
		fmt.Println("❌ 删除联系人失败:", err)
		fmt.Fprintf(w, `{"success":false,"message":"删除失败：%s"}`, err.Error())
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		fmt.Println("❌ 错误：无权限删除该联系人")
		fmt.Fprint(w, `{"success":false,"message":"联系人不存在或无权限删除"}`)
		return
	}

	fmt.Println("✅ 联系人删除成功！")
	fmt.Fprint(w, `{"success":true,"message":"联系人删除成功！"}`)
}

// 创建数据表（新增联系人表）
func createTables() {
	// 用户表
	_, _ = db.Exec(`CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT UNIQUE NOT NULL,
		password TEXT NOT NULL,
		email TEXT UNIQUE NOT NULL
	)`)
	// 邮件表
	_, _ = db.Exec(`CREATE TABLE IF NOT EXISTS emails (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		to_email TEXT NOT NULL,
		from_email TEXT NOT NULL,
		subject TEXT NOT NULL,
		content TEXT NOT NULL
	)`)
	// 🔥 新增：联系人表（核心：user_email 绑定所属用户，实现数据隔离）
	_, _ = db.Exec(`CREATE TABLE IF NOT EXISTS contacts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_email TEXT NOT NULL,
		contact_name TEXT NOT NULL,
		contact_email TEXT UNIQUE NOT NULL,
		contact_note TEXT DEFAULT '',
		UNIQUE(user_email, contact_email)
	)`)
}