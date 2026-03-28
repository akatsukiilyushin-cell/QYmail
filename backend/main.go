package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

var db *sql.DB

func setCORS(w http.ResponseWriter, r *http.Request) bool {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusNoContent)
		return true
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	return false
}

func main() {
	var err error
	db, err = sql.Open("sqlite", "./qymail.db")
	if err != nil {
		fmt.Println("❌ 数据库打开失败:", err)
		return
	}
	defer db.Close()

	err = db.Ping()
	if err != nil {
		fmt.Println("❌ 数据库连接失败:", err)
		return
	}

	createTables()
	fmt.Println("✅ 数据库初始化成功！")
	fmt.Println("✅ QYmail Pro 启动成功！http://127.0.0.1:8080")

	// 用户相关
	http.HandleFunc("/api/register", registerHandler)
	http.HandleFunc("/api/login", loginHandler)
	http.HandleFunc("/api/logout", logoutHandler)
	http.HandleFunc("/api/settings", settingsHandler)
	http.HandleFunc("/api/settings/update", updateSettingsHandler)

	// 邮件相关
	http.HandleFunc("/api/emails", emailsHandler)
	http.HandleFunc("/api/send", sendHandler)
	http.HandleFunc("/api/detail", detailHandler)
	http.HandleFunc("/api/delete", deleteHandler)
	http.HandleFunc("/api/mail/star", starMailHandler)
	http.HandleFunc("/api/mail/important", importantMailHandler)
	http.HandleFunc("/api/mail/read", markReadHandler)
	http.HandleFunc("/api/search", searchHandler)

	// 联系人相关
	http.HandleFunc("/api/contact/add", contactAddHandler)
	http.HandleFunc("/api/contact/list", contactListHandler)
	http.HandleFunc("/api/contact/edit", contactEditHandler)
	http.HandleFunc("/api/contact/delete", contactDeleteHandler)

	// 统计相关
	http.HandleFunc("/api/stats", statsHandler)
	http.HandleFunc("/api/backup", backupHandler)

	http.ListenAndServe("127.0.0.1:8080", nil)
}

// ==================== 用户注册 ====================
func registerHandler(w http.ResponseWriter, r *http.Request) {
	if setCORS(w, r) { return }
	if r.Method != "POST" {
		fmt.Fprint(w, `{"success":false,"message":"仅支持POST"}`)
		return
	}

	err := r.ParseForm()
	if err != nil {
		fmt.Fprint(w, `{"success":false,"message":"参数解析失败"}`)
		return
	}

	username := strings.TrimSpace(r.FormValue("username"))
	password := strings.TrimSpace(r.FormValue("password"))
	userEmail := username + "@qy.com"

	if username == "" || password == "" {
		fmt.Fprint(w, `{"success":false,"message":"用户名密码不能为空"}`)
		return
	}

	var count int
	db.QueryRow("SELECT COUNT(*) FROM users WHERE username=?", username).Scan(&count)
	if count > 0 {
		fmt.Fprint(w, `{"success":false,"message":"用户名已被注册"}`)
		return
	}

	_, err = db.Exec("INSERT INTO users (username,password,email,created_at) VALUES (?,?,?,?)",
		username, password, userEmail, time.Now())
	if err != nil {
		fmt.Fprint(w, `{"success":false,"message":"注册失败"}`)
		return
	}

	db.Exec("INSERT INTO emails (to_email,from_email,subject,content,created_at) VALUES (?,?,?,?,?)",
		userEmail, "admin@qy.com", "欢迎使用QYmail Pro", "你的专属邮箱已开通！", time.Now())

	fmt.Printf("✅ 新用户注册成功：%s\n", userEmail)
	fmt.Fprintf(w, `{"success":true,"email":"%s","message":"注册成功"}`, userEmail)
}

// ==================== 用户登录 ====================
func loginHandler(w http.ResponseWriter, r *http.Request) {
	if setCORS(w, r) { return }
	if r.Method != "POST" {
		fmt.Fprint(w, `{"success":false,"message":"仅支持POST"}`)
		return
	}

	err := r.ParseForm()
	if err != nil {
		fmt.Fprint(w, `{"success":false,"message":"参数解析失败"}`)
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")

	var dbPwd, userEmail string
	err = db.QueryRow("SELECT password,email FROM users WHERE username=?", username).
		Scan(&dbPwd, &userEmail)
	
	if err != nil || dbPwd != password {
		fmt.Fprint(w, `{"success":false,"message":"用户名或密码错误"}`)
		return
	}

	db.Exec("INSERT INTO login_logs (email,ip_address,login_time) VALUES (?,?,?)",
		userEmail, getClientIP(r), time.Now())

	fmt.Printf("✅ 用户登录成功：%s\n", userEmail)
	fmt.Fprintf(w, `{"success":true,"email":"%s","message":"登录成功"}`, userEmail)
}

// ==================== 登出 ====================
func logoutHandler(w http.ResponseWriter, r *http.Request) {
	if setCORS(w, r) { return }
	if r.Method != "POST" {
		fmt.Fprint(w, `{"success":false,"message":"仅支持POST"}`)
		return
	}

	userEmail := r.FormValue("user_email")
	fmt.Printf("👋 用户 %s 已退出登录\n", userEmail)
	fmt.Fprint(w, `{"success":true,"message":"退出登录成功"}`)
}

// ==================== 用户设置 ====================
func settingsHandler(w http.ResponseWriter, r *http.Request) {
	if setCORS(w, r) { return }

	userEmail := r.URL.Query().Get("user_email")
	if userEmail == "" {
		fmt.Fprint(w, `{"success":false,"message":"参数缺失"}`)
		return
	}

	var nickname string
	var notifySound, autoSync int
	
	db.QueryRow("SELECT nickname,notify_sound,auto_sync FROM settings WHERE user_email=?", 
		userEmail).Scan(&nickname, &notifySound, &autoSync)

	fmt.Fprintf(w, `{"success":true,"nickname":"%s","notifySound":%d,"autoSync":%d}`,
		nickname, notifySound, autoSync)
}

func updateSettingsHandler(w http.ResponseWriter, r *http.Request) {
	if setCORS(w, r) { return }
	if r.Method != "POST" {
		fmt.Fprint(w, `{"success":false,"message":"仅支持POST"}`)
		return
	}

	err := r.ParseForm()
	if err != nil {
		fmt.Fprint(w, `{"success":false,"message":"参数解析失败"}`)
		return
	}

	userEmail := r.FormValue("user_email")
	nickname := r.FormValue("nickname")
	notifySound := r.FormValue("notify_sound")
	autoSync := r.FormValue("auto_sync")

	db.Exec(`INSERT OR REPLACE INTO settings 
		(user_email,nickname,notify_sound,auto_sync) 
		VALUES (?,?,?,?)`,
		userEmail, nickname, notifySound, autoSync)

	fmt.Fprint(w, `{"success":true,"message":"设置已保存"}`)
}

// ==================== 获取邮件 ====================
func emailsHandler(w http.ResponseWriter, r *http.Request) {
	if setCORS(w, r) { return }

	userEmail := r.URL.Query().Get("user_email")
	tab := r.URL.Query().Get("tab")
	if tab == "" {
		tab = "inbox"
	}

	if userEmail == "" {
		fmt.Fprint(w, `{"success":false,"message":"参数缺失"}`)
		return
	}

	query := "SELECT id,from_email,subject,content,tags,is_draft,starred,important,unread FROM emails WHERE "
	
	if tab == "drafts" {
		query += "from_email=? AND is_draft=1"
	} else if tab == "sent" {
		query += "from_email=? AND is_draft=0 AND to_email!=''"
	} else {
		query += "to_email=? AND is_draft=0"
	}
	
	query += " ORDER BY created_at DESC"

	rows, err := db.Query(query, userEmail)
	if err != nil {
		fmt.Fprintf(w, `{"success":false,"message":"查询失败"}`)
		return
	}
	defer rows.Close()

	var emails []string
	for rows.Next() {
		var id int
		var from, subject, content, tags string
		var isDraft, starred, important, unread int
		
		err := rows.Scan(&id, &from, &subject, &content, &tags, &isDraft, &starred, &important, &unread)
		if err != nil {
			continue
		}

		from = escapeJSON(from)
		subject = escapeJSON(subject)
		content = escapeJSON(content)
		tags = escapeJSON(tags)

		emails = append(emails, fmt.Sprintf(
			`{"id":%d,"from":"%s","subject":"%s","preview":"%s","content":"%s","tags":"%s","is_draft":%d,"starred":%d,"important":%d,"unread":%d}`,
			id, from, subject, subject, content, tags, isDraft, starred, important, unread))
	}

	result := `{"success":true,"emails":[`
	if len(emails) > 0 {
		result += strings.Join(emails, ",")
	}
	result += `]}`
	fmt.Fprint(w, result)
}

// ==================== 发送邮件 ====================
func sendHandler(w http.ResponseWriter, r *http.Request) {
	if setCORS(w, r) { return }
	if r.Method != "POST" {
		fmt.Fprint(w, `{"success":false,"message":"仅支持POST"}`)
		return
	}

	err := r.ParseForm()
	if err != nil {
		fmt.Fprint(w, `{"success":false,"message":"参数解析失败"}`)
		return
	}

	fromEmail := strings.TrimSpace(r.FormValue("from_email"))
	toEmail := strings.TrimSpace(r.FormValue("to_email"))
	subject := strings.TrimSpace(r.FormValue("subject"))
	content := strings.TrimSpace(r.FormValue("content"))
	tags := strings.TrimSpace(r.FormValue("tags"))
	isDraft := 0
	if r.FormValue("is_draft") == "1" {
		isDraft = 1
	}

	if fromEmail == "" || subject == "" {
		fmt.Fprint(w, `{"success":false,"message":"参数缺失"}`)
		return
	}

	if isDraft == 0 && toEmail == "" {
		fmt.Fprint(w, `{"success":false,"message":"收件人不能为空"}`)
		return
	}

	var userCount int
	db.QueryRow("SELECT COUNT(*) FROM users WHERE email=?", fromEmail).Scan(&userCount)
	if userCount == 0 {
		fmt.Fprint(w, `{"success":false,"message":"发件人不存在"}`)
		return
	}

	if isDraft == 0 {
		var toCount int
		db.QueryRow("SELECT COUNT(*) FROM users WHERE email=?", toEmail).Scan(&toCount)
		if toCount == 0 {
			fmt.Fprint(w, `{"success":false,"message":"收件人不存在"}`)
			return
		}
	}

	_, err = db.Exec("INSERT INTO emails (to_email,from_email,subject,content,tags,is_draft,created_at) VALUES (?,?,?,?,?,?,?)",
		toEmail, fromEmail, subject, content, tags, isDraft, time.Now())
	if err != nil {
		fmt.Fprint(w, `{"success":false,"message":"发送失败"}`)
		return
	}

	if isDraft == 1 {
		fmt.Fprint(w, `{"success":true,"message":"草稿已保存"}`)
	} else {
		fmt.Fprint(w, `{"success":true,"message":"邮件已发送"}`)
	}
}

// ==================== 邮件详情 ====================
func detailHandler(w http.ResponseWriter, r *http.Request) {
	if setCORS(w, r) { return }

	emailID := r.URL.Query().Get("id")
	userEmail := r.URL.Query().Get("user_email")

	if emailID == "" || userEmail == "" {
		fmt.Fprint(w, `{"success":false,"message":"参数缺失"}`)
		return
	}

	id, _ := strconv.Atoi(emailID)
	var from, subject, content, tags string
	err := db.QueryRow("SELECT from_email,subject,content,tags FROM emails WHERE id=? AND (to_email=? OR from_email=?)",
		id, userEmail, userEmail).Scan(&from, &subject, &content, &tags)
	if err != nil {
		fmt.Fprint(w, `{"success":false,"message":"邮件不存在"}`)
		return
	}

	from = escapeJSON(from)
	subject = escapeJSON(subject)
	content = escapeJSON(content)
	tags = escapeJSON(tags)

	fmt.Fprintf(w, `{"success":true,"from":"%s","subject":"%s","content":"%s","tags":"%s"}`,
		from, subject, content, tags)
}

// ==================== 删除邮件 ====================
func deleteHandler(w http.ResponseWriter, r *http.Request) {
	if setCORS(w, r) { return }
	if r.Method != "POST" {
		fmt.Fprint(w, `{"success":false,"message":"仅支持POST"}`)
		return
	}

	err := r.ParseForm()
	if err != nil {
		fmt.Fprint(w, `{"success":false,"message":"参数解析失败"}`)
		return
	}

	emailID := r.FormValue("id")
	userEmail := r.FormValue("user_email")

	id, _ := strconv.Atoi(emailID)
	_, err = db.Exec("DELETE FROM emails WHERE id=? AND to_email=?", id, userEmail)
	if err != nil {
		fmt.Fprint(w, `{"success":false,"message":"删除失败"}`)
		return
	}

	fmt.Fprint(w, `{"success":true,"message":"删除成功"}`)
}

// ==================== 星标邮件 ====================
func starMailHandler(w http.ResponseWriter, r *http.Request) {
	if setCORS(w, r) { return }
	if r.Method != "POST" {
		fmt.Fprint(w, `{"success":false,"message":"仅支持POST"}`)
		return
	}

	err := r.ParseForm()
	if err != nil {
		fmt.Fprint(w, `{"success":false,"message":"参数解析失败"}`)
		return
	}

	emailID := r.FormValue("id")
	id, _ := strconv.Atoi(emailID)
	
	var starred int
	db.QueryRow("SELECT starred FROM emails WHERE id=?", id).Scan(&starred)
	
	newStarred := 1 - starred
	db.Exec("UPDATE emails SET starred=? WHERE id=?", newStarred, id)

	fmt.Fprint(w, `{"success":true,"message":"操作成功"}`)
}

// ==================== 标记重要 ====================
func importantMailHandler(w http.ResponseWriter, r *http.Request) {
	if setCORS(w, r) { return }
	if r.Method != "POST" {
		fmt.Fprint(w, `{"success":false,"message":"仅支持POST"}`)
		return
	}

	err := r.ParseForm()
	if err != nil {
		fmt.Fprint(w, `{"success":false,"message":"参数解析失败"}`)
		return
	}

	emailID := r.FormValue("id")
	id, _ := strconv.Atoi(emailID)
	
	var important int
	db.QueryRow("SELECT important FROM emails WHERE id=?", id).Scan(&important)
	
	newImportant := 1 - important
	db.Exec("UPDATE emails SET important=? WHERE id=?", newImportant, id)

	fmt.Fprint(w, `{"success":true,"message":"操作成功"}`)
}

// ==================== 标记已读 ====================
func markReadHandler(w http.ResponseWriter, r *http.Request) {
	if setCORS(w, r) { return }
	if r.Method != "POST" {
		fmt.Fprint(w, `{"success":false,"message":"仅支持POST"}`)
		return
	}

	err := r.ParseForm()
	if err != nil {
		fmt.Fprint(w, `{"success":false,"message":"参数解析失败"}`)
		return
	}

	emailID := r.FormValue("id")
	id, _ := strconv.Atoi(emailID)
	
	db.Exec("UPDATE emails SET unread=0 WHERE id=?", id)
	fmt.Fprint(w, `{"success":true,"message":"操作成功"}`)
}

// ==================== 搜索邮件 ====================
func searchHandler(w http.ResponseWriter, r *http.Request) {
	if setCORS(w, r) { return }

	userEmail := r.URL.Query().Get("user_email")
	searchFrom := "%" + r.URL.Query().Get("from") + "%"
	searchSubject := "%" + r.URL.Query().Get("subject") + "%"
	searchContent := "%" + r.URL.Query().Get("content") + "%"

	rows, err := db.Query(`SELECT id,from_email,subject,content,tags FROM emails 
		WHERE to_email=? AND from_email LIKE ? AND subject LIKE ? AND content LIKE ?
		ORDER BY created_at DESC LIMIT 50`,
		userEmail, searchFrom, searchSubject, searchContent)
	if err != nil {
		fmt.Fprint(w, `{"success":true,"emails":[]}`)
		return
	}
	defer rows.Close()

	var emails []string
	for rows.Next() {
		var id int
		var from, subject, content, tags string
		if err := rows.Scan(&id, &from, &subject, &content, &tags); err == nil {
			from = escapeJSON(from)
			subject = escapeJSON(subject)
			emails = append(emails, fmt.Sprintf(
				`{"id":%d,"from":"%s","subject":"%s"}`, id, from, subject))
		}
	}

	result := `{"success":true,"emails":[`
	if len(emails) > 0 {
		result += strings.Join(emails, ",")
	}
	result += `]}`
	fmt.Fprint(w, result)
}

// ==================== 联系人管理 ====================
func contactAddHandler(w http.ResponseWriter, r *http.Request) {
	if setCORS(w, r) { return }
	if r.Method != "POST" {
		fmt.Fprint(w, `{"success":false,"message":"仅支持POST"}`)
		return
	}

	err := r.ParseForm()
	if err != nil {
		fmt.Fprint(w, `{"success":false,"message":"参数解析失败"}`)
		return
	}

	userEmail := r.FormValue("user_email")
	contactName := r.FormValue("contact_name")
	contactEmail := r.FormValue("contact_email")
	contactGroup := r.FormValue("contact_group")

	if userEmail == "" || contactName == "" || contactEmail == "" {
		fmt.Fprint(w, `{"success":false,"message":"参数缺失"}`)
		return
	}

	// 检查是否已存在
	var existCount int
	db.QueryRow("SELECT COUNT(*) FROM contacts WHERE user_email=? AND contact_email=?", 
		userEmail, contactEmail).Scan(&existCount)
	if existCount > 0 {
		fmt.Fprint(w, `{"success":false,"message":"该邮箱的联系人已存在"}`)
		return
	}

	// 插入新联系人
	result, err := db.Exec("INSERT INTO contacts (user_email,contact_name,contact_email,contact_group,is_vip) VALUES (?,?,?,?,?)",
		userEmail, contactName, contactEmail, contactGroup, 0)
	if err != nil {
		fmt.Printf("❌ 添加联系人失败: %v\n", err)
		fmt.Fprint(w, `{"success":false,"message":"添加失败"}`)
		return
	}

	id, _ := result.LastInsertId()
	fmt.Printf("✅ 联系人添加成功: ID=%d, 名称=%s, 邮箱=%s\n", id, contactName, contactEmail)
	fmt.Fprint(w, `{"success":true,"message":"联系人已添加"}`)
}

func contactListHandler(w http.ResponseWriter, r *http.Request) {
	if setCORS(w, r) { return }

	userEmail := r.URL.Query().Get("user_email")
	
	rows, err := db.Query("SELECT id,contact_name,contact_email,contact_group,is_vip FROM contacts WHERE user_email=? ORDER BY contact_name",
		userEmail)
	if err != nil {
		fmt.Fprint(w, `{"success":true,"contacts":[]}`)
		return
	}
	defer rows.Close()

	var contacts []string
	for rows.Next() {
		var id, isVIP int
		var name, email, group string
		if err := rows.Scan(&id, &name, &email, &group, &isVIP); err == nil {
			name = escapeJSON(name)
			email = escapeJSON(email)
			group = escapeJSON(group)
			contacts = append(contacts, fmt.Sprintf(
				`{"id":%d,"name":"%s","email":"%s","group":"%s","vip":%d}`,
				id, name, email, group, isVIP))
		}
	}

	result := `{"success":true,"contacts":[`
	if len(contacts) > 0 {
		result += strings.Join(contacts, ",")
	}
	result += `]}`
	fmt.Fprint(w, result)
}

func contactEditHandler(w http.ResponseWriter, r *http.Request) {
	if setCORS(w, r) { return }
	if r.Method != "POST" {
		fmt.Fprint(w, `{"success":false,"message":"仅支持POST"}`)
		return
	}

	err := r.ParseForm()
	if err != nil {
		fmt.Fprint(w, `{"success":false,"message":"参数解析失败"}`)
		return
	}

	contactID := r.FormValue("id")
	name := r.FormValue("contact_name")
	email := r.FormValue("contact_email")
	group := r.FormValue("contact_group")

	id, _ := strconv.Atoi(contactID)
	db.Exec("UPDATE contacts SET contact_name=?,contact_email=?,contact_group=? WHERE id=?",
		name, email, group, id)

	fmt.Fprint(w, `{"success":true,"message":"联系人已更新"}`)
}

func contactDeleteHandler(w http.ResponseWriter, r *http.Request) {
	if setCORS(w, r) { return }
	if r.Method != "POST" {
		fmt.Fprint(w, `{"success":false,"message":"仅支持POST"}`)
		return
	}

	err := r.ParseForm()
	if err != nil {
		fmt.Fprint(w, `{"success":false,"message":"参数解析失败"}`)
		return
	}

	contactID := r.FormValue("id")
	id, _ := strconv.Atoi(contactID)

	db.Exec("DELETE FROM contacts WHERE id=?", id)
	fmt.Fprint(w, `{"success":true,"message":"联系人已删除"}`)
}

// ==================== 统计 ====================
func statsHandler(w http.ResponseWriter, r *http.Request) {
	if setCORS(w, r) { return }

	userEmail := r.URL.Query().Get("user_email")

	var totalCount, unreadCount, starredCount, importantCount int
	db.QueryRow("SELECT COUNT(*) FROM emails WHERE to_email=?", userEmail).Scan(&totalCount)
	db.QueryRow("SELECT COUNT(*) FROM emails WHERE to_email=? AND unread=1", userEmail).Scan(&unreadCount)
	db.QueryRow("SELECT COUNT(*) FROM emails WHERE to_email=? AND starred=1", userEmail).Scan(&starredCount)
	db.QueryRow("SELECT COUNT(*) FROM emails WHERE to_email=? AND important=1", userEmail).Scan(&importantCount)

	fmt.Fprintf(w, `{"success":true,"totalCount":%d,"unreadCount":%d,"starredCount":%d,"importantCount":%d,"totalSpace":"1.2 GB"}`,
		totalCount, unreadCount, starredCount, importantCount)
}

func backupHandler(w http.ResponseWriter, r *http.Request) {
	if setCORS(w, r) { return }

	userEmail := r.URL.Query().Get("user_email")
	
	w.Header().Set("Content-Disposition", "attachment; filename=qymail_backup.json")
	w.Header().Set("Content-Type", "application/json")

	fmt.Fprintf(w, `{"user":"%s","backup_time":"%s","emails":[],"contacts":[]}`,
		userEmail, time.Now().Format("2006-01-02 15:04:05"))
}

// ==================== 辅助函数 ====================
func escapeJSON(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", `\r`)
	return s
}

func getClientIP(r *http.Request) string {
	ip := r.Header.Get("X-Forwarded-For")
	if ip != "" {
		return strings.Split(ip, ",")[0]
	}
	return strings.Split(r.RemoteAddr, ":")[0]
}

// ==================== 数据库初始化 ====================
func createTables() {
	// 用户表
	db.Exec(`CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT UNIQUE NOT NULL,
		password TEXT NOT NULL,
		email TEXT UNIQUE NOT NULL,
		nickname TEXT DEFAULT '',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)

	// 邮件表
	db.Exec(`CREATE TABLE IF NOT EXISTS emails (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		to_email TEXT NOT NULL,
		from_email TEXT NOT NULL,
		subject TEXT NOT NULL,
		content TEXT NOT NULL,
		tags TEXT DEFAULT '',
		is_draft INTEGER DEFAULT 0,
		starred INTEGER DEFAULT 0,
		important INTEGER DEFAULT 0,
		unread INTEGER DEFAULT 1,
		folder_id INTEGER DEFAULT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)

	// 联系人表 - 修复版
	db.Exec(`DROP TABLE IF EXISTS contacts`)
	db.Exec(`CREATE TABLE IF NOT EXISTS contacts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_email TEXT NOT NULL,
		contact_name TEXT NOT NULL,
		contact_email TEXT NOT NULL,
		contact_group TEXT DEFAULT '',
		is_vip INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)

	// 设置表
	db.Exec(`CREATE TABLE IF NOT EXISTS settings (
		user_email TEXT PRIMARY KEY,
		nickname TEXT DEFAULT '',
		notify_sound INTEGER DEFAULT 1,
		auto_sync INTEGER DEFAULT 1,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)

	// 登录日志表
	db.Exec(`CREATE TABLE IF NOT EXISTS login_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		email TEXT NOT NULL,
		ip_address TEXT DEFAULT '',
		login_time DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)

	fmt.Println("✅ 数据表初始化完成")
}