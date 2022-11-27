package service

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	database "todolist.go/db"
)

func NewUserForm(ctx *gin.Context) {
	ctx.HTML(http.StatusOK, "new_user_form.html", gin.H{"Title": "Register user"})
}

func hash(pw string) []byte {
	const salt = "todolist.go#"
	h := sha256.New()
	h.Write([]byte(salt))
	h.Write([]byte(pw))
	return h.Sum(nil)
}

func RegisterUser(ctx *gin.Context) {
	// フォームデータの受け取り
	username := ctx.PostForm("username")
	password := ctx.PostForm("password")
	password_confirm := ctx.PostForm("password_confirm")
	switch _, err := strconv.Atoi(password); {
	case username == "":
		ctx.HTML(http.StatusBadRequest, "new_user_form.html", gin.H{"Title": "Register user", "Error": "Username is not provided", "Username": username})
		return
	case password == "":
		ctx.HTML(http.StatusBadRequest, "new_user_form.html", gin.H{"Title": "Register user", "Error": "Password is not provided", "Password": password})
		return
	case password != password_confirm:
		ctx.HTML(http.StatusBadRequest, "new_user_form.html", gin.H{"Title": "Register user", "Error": "Password and password confirmation are not same", "Username": username, "Password": password, "PasswordConfirm": password_confirm})
		return
	// too short password
	case len(password) < 8:
		ctx.HTML(http.StatusBadRequest, "new_user_form.html", gin.H{"Title": "Register user", "Error": "Password is too short", "Username": username, "Password": password, "PasswordConfirm": password_confirm})
		return
	// consists of only numbers
	case err == nil:
		ctx.HTML(http.StatusBadRequest, "new_user_form.html", gin.H{"Title": "Register user", "Error": "Password consists of only numbers", "Username": username, "Password": password, "PasswordConfirm": password_confirm})
		return
	}

	// DB 接続
	db, err := database.GetConnection()
	if err != nil {
		Error(http.StatusInternalServerError, err.Error())(ctx)
		return
	}

	// 重複チェック
	var duplicate int
	err = db.Get(&duplicate, "SELECT COUNT(*) FROM users WHERE name=?", username)
	if err != nil {
		Error(http.StatusInternalServerError, err.Error())(ctx)
		return
	}
	if duplicate > 0 {
		ctx.HTML(http.StatusBadRequest, "new_user_form.html", gin.H{"Title": "Register user", "Error": "Username is already taken", "Username": username, "Password": password, "PasswordConfirm": password_confirm})
		return
	}

	// DB への保存
	result, err := db.Exec("INSERT INTO users(name, password) VALUES (?, ?)", username, hash(password))
	if err != nil {
		Error(http.StatusInternalServerError, err.Error())(ctx)
		return
	}

	// 保存状態の確認
	id, _ := result.LastInsertId()
	var user database.User
	err = db.Get(&user, "SELECT id, name, password FROM users WHERE id = ?", id)
	if err != nil {
		Error(http.StatusInternalServerError, err.Error())(ctx)
		return
	}
	ctx.Redirect(http.StatusFound, "/list")
}

const userkey = "user"

func Login(ctx *gin.Context) {
	username := ctx.PostForm("username")
	password := ctx.PostForm("password")

	db, err := database.GetConnection()
	if err != nil {
		Error(http.StatusInternalServerError, err.Error())(ctx)
		return
	}

	// ユーザの取得
	var user database.User
	err = db.Get(&user, "SELECT id, name, password, is_valid FROM users WHERE name = ?", username)
	if err != nil {
		ctx.HTML(http.StatusBadRequest, "login.html", gin.H{"Title": "Login", "Username": username, "Error": "No such user"})
		return
	}

	// パスワードの照合
	if hex.EncodeToString(user.Password) != hex.EncodeToString(hash(password)) {
		ctx.HTML(http.StatusBadRequest, "login.html", gin.H{"Title": "Login", "Username": username, "Error": "Incorrect password"})
		return
	}

	// 有効かどうか
	if user.IsValid == false {
		ctx.HTML(http.StatusBadRequest, "login.html", gin.H{"Title": "Login", "Username": username, "Error": "User is not valid"})
		return
	}

	// セッションの保存
	session := sessions.Default(ctx)
	session.Set(userkey, user.ID)
	session.Save()

	ctx.Redirect(http.StatusFound, "/list")
}

func LoginForm(ctx *gin.Context) {
	// ログインしているか確認
	if sessions.Default(ctx).Get("user") != nil {
		ctx.HTML(http.StatusBadRequest, "task_list.html", gin.H{"Title": "List of Tasks", "Error": "You are already logged in"})
		return
	}

	ctx.HTML(http.StatusOK, "login.html", gin.H{"Title": "Login"})
}

func LoginCheck(ctx *gin.Context) {
	if sessions.Default(ctx).Get(userkey) == nil {
		ctx.Redirect(http.StatusFound, "/login")
		ctx.Abort()
	} else {
		ctx.Next()
	}
}

func Logout(ctx *gin.Context) {
	session := sessions.Default(ctx)
	session.Clear()
	session.Options(sessions.Options{MaxAge: -1})
	session.Save()
	ctx.Redirect(http.StatusFound, "/")
}

func DeleteUserForm(ctx *gin.Context) {
	ctx.HTML(http.StatusOK, "delete_user_form.html", gin.H{"Title": "Delete user"})
}

func DeleteUser(ctx *gin.Context) {
	userID := sessions.Default(ctx).Get("user")

	// フォームデータの受け取り
	password := ctx.PostForm("password")
	password_confirm := ctx.PostForm("password_confirm")
	if password == "" {
		ctx.HTML(http.StatusBadRequest, "delete_user_form.html", gin.H{"Title": "Delete user", "Error": "Password is not provided", "Password": password})
		return
	}
	if password != password_confirm {
		ctx.HTML(http.StatusBadRequest, "delete_user_form.html", gin.H{"Title": "Delete user", "Error": "Password and password confirmation are not same", "Password": password, "PasswordConfirm": password_confirm})
		return
	}

	// DB 接続
	db, err := database.GetConnection()
	if err != nil {
		Error(http.StatusInternalServerError, err.Error())(ctx)
		return
	}

	// ユーザの取得
	var user database.User
	err = db.Get(&user, "SELECT id, name, password FROM users WHERE id = ?", userID)
	if err != nil {
		Error(http.StatusInternalServerError, err.Error())(ctx)
		return
	}

	// パスワードの照合
	if hex.EncodeToString(user.Password) != hex.EncodeToString(hash(password)) {
		ctx.HTML(http.StatusBadRequest, "delete_user_form.html", gin.H{"Title": "Delete user", "Error": "Incorrect password", "Password": password, "PasswordConfirm": password_confirm})
		return
	}

	// ユーザの削除
	_, err = db.Exec("UPDATE users SET is_valid = false WHERE id = ?", userID)
	if err != nil {
		Error(http.StatusInternalServerError, err.Error())(ctx)
		return
	}

	// セッションの削除
	session := sessions.Default(ctx)
	session.Clear()
	session.Options(sessions.Options{Path: "/", MaxAge: -1})
	session.Save()

	ctx.Redirect(http.StatusFound, "/")
}

func EditUserForm(ctx *gin.Context) {
	userID := sessions.Default(ctx).Get("user")
	db, err := database.GetConnection()
	if err != nil {
		Error(http.StatusInternalServerError, err.Error())(ctx)
		return
	}

	// ユーザの取得
	var user database.User
	err = db.Get(&user, "SELECT id, name, password, is_valid FROM users WHERE id = ?", userID)
	if err != nil {
		Error(http.StatusInternalServerError, err.Error())(ctx)
		return
	}

	ctx.HTML(http.StatusOK, "edit_user_form.html", gin.H{"Title": "Edit user", "Username": user.Name})
}

func EditUser(ctx *gin.Context) {
	userID := sessions.Default(ctx).Get("user")

	// フォームデータの受け取り
	username := ctx.PostForm("username")
	password := ctx.PostForm("password")
	password_new := ctx.PostForm("password_new")
	if username == "" {
		ctx.HTML(http.StatusBadRequest, "edit_user_form.html", gin.H{"Title": "Edit user", "Error": "Username is not provided", "Username": username})
		return
	}
	if password == "" {
		ctx.HTML(http.StatusBadRequest, "edit_user_form.html", gin.H{"Title": "Edit user", "Error": "Password is not provided", "Username": username, "Password": password})
		return
	}

	if password_new == "" {
		ctx.HTML(http.StatusBadRequest, "edit_user_form.html", gin.H{"Title": "Edit user", "Error": "New password is not provided", "Username": username, "Password": password})
		return
	}

	if password_new != "" && len(password_new) < 8 {
		ctx.HTML(http.StatusBadRequest, "edit_user_form.html", gin.H{"Title": "Edit user", "Error": "Password is too short", "Username": username, "Password": password})
		return
	}

	// DB 接続
	db, err := database.GetConnection()
	if err != nil {
		Error(http.StatusInternalServerError, err.Error())(ctx)
		return
	}

	// ユーザの取得
	var user database.User
	err = db.Get(&user, "SELECT id, name, password FROM users WHERE id = ?", userID)
	if err != nil {
		Error(http.StatusInternalServerError, err.Error())(ctx)
		return
	}

	// パスワードの照合
	if hex.EncodeToString(user.Password) != hex.EncodeToString(hash(password)) {
		ctx.HTML(http.StatusBadRequest, "edit_user_form.html", gin.H{"Title": "Edit user", "Error": "Incorrect password", "Username": username})
		return
	}

	// ユーザの更新
	_, err = db.Exec("UPDATE users SET name = ?, password = ? WHERE id = ?", username, hash(password_new), userID)
	if err != nil {
		Error(http.StatusInternalServerError, err.Error())(ctx)
		return
	}
	ctx.Redirect(http.StatusFound, "/")
}
