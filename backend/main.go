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
	fmt.Println("✅ QYmail Pro 完整版启动成功！http://127.0.0.1:8080")

	// ==================== 用户相关 ====================
	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/api/register", registerHandler)
	http.HandleFunc("/api/login", loginHandler)
	http.HandleFunc("/api/logout", logoutHandler)
	http.HandleFunc("/api/verify-2fa", verify2FAHandler)
	http.HandleFunc("/api/settings", settingsHandler)
	http.HandleFunc("/api/settings/update", updateSettingsHandler)

	// ==================== 邮件相关 ====================
	http.HandleFunc("/api/emails", emailsHandler)
	http.HandleFunc("/api/send", sendHandler)
	http.HandleFunc("/api/detail", detailHandler)
	http.HandleFunc("/api/delete", deleteHandler)
	http.HandleFunc("/api/mail/star", starMailHandler)
	http.HandleFunc("/api/mail/important", importantMailHandler)
	http.HandleFunc("/api/mail/read", markReadHandler)
	http.HandleFunc("/api/search", searchHandler)

	// ==================== 文件夹相关 ====================
	http.HandleFunc("/api/folders", getFoldersHandler)
	http.HandleFunc("/api/folder/create", createFolderHandler)
	http.HandleFunc("/api/folder/delete", deleteFolderHandler)
	http.HandleFunc("/api/mail/move", moveMailHandler)

	// ==================== 联系人相关 ====================
	http.HandleFunc("/api/contact/add", contactAddHandler)
	http.HandleFunc("/api/contact/list", contactListHandler)
	http.HandleFunc("/api/contact/edit", contactEditHandler)
	http.HandleFunc("/api/contact/delete", contactDeleteHandler)

	// ==================== 统计相关 ====================
	http.HandleFunc("/api/stats", statsHandler)
	http.HandleFunc("/api/backup", backupHandler)

	// ==================== 标签相关 ====================
	http.HandleFunc("/api/tags", tagsHandler)

	http.ListenAndServe("127.0.0.1:8080", nil)
}

// ==================== 首页 ====================
func homeHandler(w http.ResponseWriter, r *http.Request) {
	if setCORS(w, r) { return }
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, `<h1 style="color:#d90429;">QYmail Pro 完整版已启动</h1>`)
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

	// 添加欢迎邮件
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
	var twoFAEnabled int
	err = db.QueryRow("SELECT password,email,two_fa_enabled FROM users WHERE username=?", username).
		Scan(&dbPwd, &userEmail, &twoFAEnabled)
	
	if err != nil || dbPwd != password {
		fmt.Fprint(w, `{"success":false,"message":"用户名或密码错误"}`)
		return
	}

	// 记录登录日志
	db.Exec("INSERT INTO login_logs (email,ip_address,login_time) VALUES (?,?,?)",
		userEmail, getClientIP(r), time.Now())

	fmt.Printf("✅ 用户登录成功：%s\n", userEmail)
	
	if twoFAEnabled == 1 {
		fmt.Fprintf(w, `{"success":true,"email":"%s","requires_2fa":true,"message":"需要2FA验证"}`, userEmail)
	} else {
		fmt.Fprintf(w, `{"success":true,"email":"%s","requires_2fa":false,"message":"登录成功"}`, userEmail)
	}
}

// ==================== 2FA 验证 ====================
func verify2FAHandler(w http.ResponseWriter, r *http.Request) {
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
	otpCode := r.FormValue("otp_code")

	var userEmail, secretKey string
	err = db.QueryRow("SELECT email,two_fa_secret FROM users WHERE username=?", username).
		Scan(&userEmail, &secretKey)
	
	if err != nil {
		fmt.Fprint(w, `{"success":false,"message":"用户不存在"}`)
		return
	}

	// TODO: 实现 TOTP 验证逻辑
	// 这里简化为直接验证（实际应使用 TOTP 库）
	if otpCode == "123456" { // 演示用，实际应验证真实的 OTP
		fmt.Fprintf(w, `{"success":true,"email":"%s","message":"验证成功"}`, userEmail)
	} else {
		fmt.Fprint(w, `{"success":false,"message":"验证码错误"}`)
	}
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

	var nickname, signature string
	var notifySound, autoSync int
	
	db.QueryRow("SELECT nickname,signature,notify_sound,auto_sync FROM settings WHERE user_email=?", 
		userEmail).Scan(&nickname, &signature, &notifySound, &autoSync)

	fmt.Fprintf(w, `{"success":true,"nickname":"%s","signature":"%s","notifySound":%d,"autoSync":%d}`,
		nickname, signature, notifySound, autoSync)
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
	signature := r.FormValue("signature")
	notifySound := r.FormValue("notify_sound")
	autoSync := r.FormValue("auto_sync")

	// 更新或插入设置
	db.Exec(`INSERT OR REPLACE INTO settings 
		(user_email,nickname,signature,notify_sound,auto_sync) 
		VALUES (?,?,?,?,?)`,
		userEmail, nickname, signature, notifySound, autoSync)

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

		preview := content
		if len(preview) > 50 {
			preview = preview[:50] + "..."
		}

		from = escapeJSON(from)
		subject = escapeJSON(subject)
		preview = escapeJSON(preview)
		tags = escapeJSON(tags)

		emails = append(emails, fmt.Sprintf(
			`{"id":%d,"from":"%s","subject":"%s","preview":"%s","content":"%s","tags":"%s","is_draft":%d,"starred":%d,"important":%d,"unread":%d}`,
			id, from, subject, preview, content, tags, isDraft, starred, important, unread))
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

// ==================== 文件夹管理 ====================
func getFoldersHandler(w http.ResponseWriter, r *http.Request) {
	if setCORS(w, r) { return }

	userEmail := r.URL.Query().Get("user_email")
	
	rows, err := db.Query("SELECT id,name FROM folders WHERE user_email=?", userEmail)
	if err != nil {
		fmt.Fprint(w, `{"success":true,"folders":[]}`)
		return
	}
	defer rows.Close()

	var folders []string
	for rows.Next() {
		var id int
		var name string
		if err := rows.Scan(&id, &name); err == nil {
			folders = append(folders, fmt.Sprintf(`{"id":%d,"name":"%s"}`, id, name))
		}
	}

	result := `{"success":true,"folders":[`
	if len(folders) > 0 {
		result += strings.Join(folders, ",")
	}
	result += `]}`
	fmt.Fprint(w, result)
}

func createFolderHandler(w http.ResponseWriter, r *http.Request) {
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
	name := r.FormValue("name")

	if userEmail == "" || name == "" {
		fmt.Fprint(w, `{"success":false,"message":"参数缺失"}`)
		return
	}

	_, err = db.Exec("INSERT INTO folders (user_email,name) VALUES (?,?)", userEmail, name)
	if err != nil {
		fmt.Fprint(w, `{"success":false,"message":"创建失败"}`)
		return
	}

	fmt.Fprint(w, `{"success":true,"message":"文件夹创建成功"}`)
}

func deleteFolderHandler(w http.ResponseWriter, r *http.Request) {
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

	folderID := r.FormValue("id")
	id, _ := strconv.Atoi(folderID)

	db.Exec("DELETE FROM folders WHERE id=?", id)
	fmt.Fprint(w, `{"success":true,"message":"文件夹已删除"}`)
}

func moveMailHandler(w http.ResponseWriter, r *http.Request) {
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

	mailID := r.FormValue("mail_id")
	folderID := r.FormValue("folder_id")

	id, _ := strconv.Atoi(mailID)
	fid, _ := strconv.Atoi(folderID)

	db.Exec("UPDATE emails SET folder_id=? WHERE id=?", fid, id)
	fmt.Fprint(w, `{"success":true,"message":"移动成功"}`)
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
	isVIP := r.FormValue("is_vip")

	if userEmail == "" || contactName == "" || contactEmail == "" {
		fmt.Fprint(w, `{"success":false,"message":"参数缺失"}`)
		return
	}

	_, err = db.Exec("INSERT INTO contacts (user_email,contact_name,contact_email,contact_group,is_vip) VALUES (?,?,?,?,?)",
		userEmail, contactName, contactEmail, contactGroup, isVIP)
	if err != nil {
		fmt.Fprint(w, `{"success":false,"message":"添加失败"}`)
		return
	}

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

// ==================== 统计和分析 ====================
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

// ==================== 标签管理 ====================
func tagsHandler(w http.ResponseWriter, r *http.Request) {
	if setCORS(w, r) { return }

	userEmail := r.URL.Query().Get("user_email")
	
	rows, err := db.Query("SELECT DISTINCT tags FROM emails WHERE to_email=? AND tags!=''", userEmail)
	if err != nil {
		fmt.Fprint(w, `{"success":true,"tags":[]}`)
		return
	}
	defer rows.Close()

	tagsMap := make(map[string]bool)
	for rows.Next() {
		var tags string
		if err := rows.Scan(&tags); err == nil && tags != "" {
			for _, tag := range strings.Split(tags, ",") {
				tag = strings.TrimSpace(tag)
				if tag != "" {
					tagsMap[tag] = true
				}
			}
		}
	}

	var tagsList []string
	for tag := range tagsMap {
		tagsList = append(tagsList, fmt.Sprintf(`"%s"`, tag))
	}

	result := `{"success":true,"tags":[`
	if len(tagsList) > 0 {
		result += strings.Join(tagsList, ",")
	}
	result += `]}`
	fmt.Fprint(w, result)
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
		two_fa_enabled INTEGER DEFAULT 0,
		two_fa_secret TEXT DEFAULT '',
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

	// 联系人表
	db.Exec(`CREATE TABLE IF NOT EXISTS contacts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_email TEXT NOT NULL,
		contact_name TEXT NOT NULL,
		contact_email TEXT NOT NULL,
		contact_group TEXT DEFAULT '',
		is_vip INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(user_email, contact_email)
	)`)

	// 文件夹表
	db.Exec(`CREATE TABLE IF NOT EXISTS folders (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_email TEXT NOT NULL,
		name TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(user_email, name)
	)`)

	// 设置表
	db.Exec(`CREATE TABLE IF NOT EXISTS settings (
		user_email TEXT PRIMARY KEY,
		nickname TEXT DEFAULT '',
		signature TEXT DEFAULT '',
		notify_sound INTEGER DEFAULT 1,
		auto_sync INTEGER DEFAULT 1,
		auto_reply TEXT DEFAULT '',
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