package service

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	database "todolist.go/db"
)

// TaskList renders list of tasks in DB
func TaskList(ctx *gin.Context) {
	var now_page int
	userID := sessions.Default(ctx).Get("user")

	// Get DB connection
	db, err := database.GetConnection()
	if err != nil {
		Error(http.StatusInternalServerError, err.Error())(ctx)
		return
	}

	// Get query parameter
	kw := ctx.Query("kw")

	status := ctx.Query("status")

	tmp := ctx.Query("page")
	if tmp == "" {
		now_page = 1
	} else {
		now_page, _ = strconv.Atoi(tmp)
	}

	// Get tasks in DB
	var tasks []database.Task
	query := "SELECT id, title, created_at, is_done, priority, deadline FROM tasks INNER JOIN ownership ON task_id = id WHERE user_id = ?"
	switch {
	case kw != "" && status != "":
		err = db.Select(&tasks, query+" AND title LIKE ? AND is_done = ?", userID, "%"+kw+"%", status == "done")
	case kw != "":
		err = db.Select(&tasks, query+" AND title LIKE ?", userID, "%"+kw+"%")
	case status != "":
		err = db.Select(&tasks, query+" AND is_done = ?", userID, status == "done")
	default:
		err = db.Select(&tasks, query, userID)
	}

	len := len(tasks)
	PageLen := int(len / 10)
	if len%10 != 0 {
		PageLen++
	}
	if !(PageLen == 0) {
		if now_page >= PageLen {
			tasks = tasks[(PageLen-1)*10:]
		} else {
			tasks = tasks[(now_page-1)*10 : now_page*10]
		}
	}
	Pages := []int{}
	for i := 1; i <= PageLen; i++ {
		Pages = append(Pages, i)
	}

	// Render tasks
	ctx.HTML(http.StatusOK, "task_list.html", gin.H{"Title": "Task list", "Tasks": tasks, "Kw": kw, "Status": status, "NowPage": now_page, "Pages": Pages})
}

// ShowTask renders a task with given ID
func ShowTask(ctx *gin.Context) {
	// Get DB connection
	db, err := database.GetConnection()
	if err != nil {
		Error(http.StatusInternalServerError, err.Error())(ctx)
		return
	}

	// parse ID given as a parameter
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		Error(http.StatusBadRequest, err.Error())(ctx)
		return
	}

	// Get a task with given ID
	var task database.Task
	err = db.Get(&task, "SELECT * FROM tasks WHERE id=?", id) // Use DB#Get for one entry
	if err != nil {
		Error(http.StatusBadRequest, err.Error())(ctx)
		return
	}

	// Render task
	//ctx.String(http.StatusOK, task.Title)  // Modify it!!
	ctx.HTML(http.StatusOK, "task.html", task)
}

func NewTaskForm(ctx *gin.Context) {
	ctx.HTML(http.StatusOK, "form_new_task.html", gin.H{"Title": "Task registration"})
}

func RegisterTask(ctx *gin.Context) {
	userID := sessions.Default(ctx).Get("user")
	// Get task title
	title, exist := ctx.GetPostForm("title")
	if !exist {
		Error(http.StatusBadRequest, "No title is given")(ctx)
		return
	}
	description, exist := ctx.GetPostForm("description")
	if !exist {
		Error(http.StatusBadRequest, "No description is given")(ctx)
		return
	}
	priority, exist := ctx.GetPostForm("priority")
	if !exist {
		Error(http.StatusBadRequest, "No priority is given")(ctx)
		return
	}
	deadline, exist := ctx.GetPostForm("deadline")
	if !exist {
		Error(http.StatusBadRequest, "No deadline is given")(ctx)
		return
	}

	// Get DB connection
	db, err := database.GetConnection()
	if err != nil {
		Error(http.StatusInternalServerError, err.Error())(ctx)
		return
	}
	tx := db.MustBegin()
	result, err := tx.Exec("INSERT INTO tasks (title, description, priority, deadline) VALUES (?, ?, ?, ?)", title, description, priority, deadline)
	if err != nil {
		tx.Rollback()
		Error(http.StatusInternalServerError, err.Error())(ctx)
		return
	}

	taskID, err := result.LastInsertId()
	if err != nil {
		tx.Rollback()
		Error(http.StatusInternalServerError, err.Error())(ctx)
		return
	}
	_, err = tx.Exec("INSERT INTO ownership (user_id, task_id) VALUES (?, ?)", userID, taskID)
	if err != nil {
		tx.Rollback()
		Error(http.StatusInternalServerError, err.Error())(ctx)
		return
	}
	tx.Commit()
	ctx.Redirect(http.StatusFound, fmt.Sprintf("/task/%d", taskID))
}

func EditTaskForm(ctx *gin.Context) {
	userID := sessions.Default(ctx).Get("user")
	// ID の取得
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		Error(http.StatusBadRequest, err.Error())(ctx)
		return
	}
	// Get DB connection
	db, err := database.GetConnection()
	if err != nil {
		Error(http.StatusInternalServerError, err.Error())(ctx)
		return
	}

	// Get a owner with given ID
	var owner uint64
	err = db.Get(&owner, "SELECT user_id FROM ownership WHERE task_id=?", id) // Use DB#Get for one entry
	if err != nil {
		Error(http.StatusBadRequest, err.Error())(ctx)
		return
	}
	if owner != userID {
		Error(http.StatusBadRequest, "You are not owner of this task")(ctx)
		return
	}

	// Get target task
	var task database.Task
	err = db.Get(&task, "SELECT * FROM tasks WHERE id=?", id)
	if err != nil {
		Error(http.StatusBadRequest, err.Error())(ctx)
		return
	}
	// Render edit form
	ctx.HTML(http.StatusOK, "form_edit_task.html",
		gin.H{"Title": fmt.Sprintf("Edit task %d", task.ID), "Task": task})
}

func EditTask(ctx *gin.Context) {
	userID := sessions.Default(ctx).Get("user")
	// ID の取得
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		Error(http.StatusBadRequest, err.Error())(ctx)
		return
	}
	// Get DB connection
	db, err := database.GetConnection()
	if err != nil {
		Error(http.StatusInternalServerError, err.Error())(ctx)
		return
	}
	// Get target task
	var task database.Task
	err = db.Get(&task, "SELECT * FROM tasks WHERE id=?", id)
	if err != nil {
		Error(http.StatusBadRequest, err.Error())(ctx)
		return
	}

	// Get a owner with given ID
	var owner uint64
	err = db.Get(&owner, "SELECT user_id FROM ownership WHERE task_id=?", id) // Use DB#Get for one entry
	if err != nil {
		Error(http.StatusBadRequest, err.Error())(ctx)
		return
	}
	if owner != userID {
		Error(http.StatusBadRequest, "You are not owner of this task")(ctx)
		return
	}

	// Update task
	task.Title, _ = ctx.GetPostForm("title")
	task.Description, _ = ctx.GetPostForm("description")
	tmp, _ := ctx.GetPostForm("is_done")
	task.IsDone, _ = strconv.ParseBool(tmp)
	tmp, _ = ctx.GetPostForm("priority")
	task.Priority, _ = strconv.ParseUint(tmp, 10, 64)
	deadline, _ := ctx.GetPostForm("deadline")
	_, err = db.Exec("UPDATE tasks SET title=?, description=?, is_done=?, priority=?, deadline=? WHERE id=?",
		task.Title, task.Description, task.IsDone, task.Priority, deadline, task.ID)
	if err != nil {
		Error(http.StatusInternalServerError, err.Error())(ctx)
		return
	}
	// Render status
	ctx.Redirect(http.StatusFound, fmt.Sprintf("/task/%d", task.ID))
}

func DeleteTask(ctx *gin.Context) {
	// ID の取得
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		Error(http.StatusBadRequest, err.Error())(ctx)
		return
	}
	// Get DB connection
	db, err := database.GetConnection()
	if err != nil {
		Error(http.StatusInternalServerError, err.Error())(ctx)
		return
	}
	// Delete the task from DB
	_, err = db.Exec("DELETE FROM tasks WHERE id=?", id)
	if err != nil {
		Error(http.StatusInternalServerError, err.Error())(ctx)
		return
	}
	// Redirect to /list
	ctx.Redirect(http.StatusFound, "/list")
}

func CheckUser(ctx *gin.Context) {
	// ID の取得
	id, err := strconv.Atoi(ctx.Param("id"))

	// Get DB connection
	db, err := database.GetConnection()
	if err != nil {
		Error(http.StatusInternalServerError, err.Error())(ctx)
		return
	}
	var userID uint64
	db.Get(&userID, "SELECT user_id FROM tasks INNER JOIN ownership ON task_id = id WHERE task_id=?", id)
	if sessions.Default(ctx).Get("user") != userID {
		ctx.HTML(http.StatusBadRequest, "error.html", gin.H{"Code": "401 Unauthorized", "Error": "You are not the owner of this task."})
		ctx.Abort()
	} else {
		ctx.Next()
	}
}
