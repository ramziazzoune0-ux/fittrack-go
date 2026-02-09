package main

import (
	"database/sql"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"

	_ "modernc.org/sqlite"
)

type RoutineRecord struct {
	ID, Duration         int
	Date, Category, Meal string
}

type WorkoutCategory struct {
	ID   int
	Name string
}

type DashboardStats struct {
	TotalWorkouts, TotalMinutes int
	MostTrained                 string
}

type ProgressReport struct {
	Score       int
	Level       string
	FoodStatus  string
	Consistency int
}

type PageData struct {
	Categories []WorkoutCategory
	CatNames   []string
	History    []RoutineRecord
	Stats      DashboardStats
	Progress   ProgressReport
}

var db *sql.DB

// Logic functions
func getStats() DashboardStats {
	var s DashboardStats
	db.QueryRow(`SELECT COUNT(*) FROM daily_routine WHERE date(workout_date) >= date('now', '-7 days')`).Scan(&s.TotalWorkouts)
	db.QueryRow(`SELECT COALESCE(SUM(duration), 0) FROM daily_routine WHERE date(workout_date) >= date('now', '-7 days')`).Scan(&s.TotalMinutes)
	db.QueryRow(`SELECT category FROM daily_routine WHERE date(workout_date) >= date('now', '-7 days') GROUP BY category ORDER BY COUNT(*) DESC LIMIT 1`).Scan(&s.MostTrained)
	if s.MostTrained == "" {
		s.MostTrained = "None"
	}
	return s
}

func main() {
	var err error
	db, err = sql.Open("sqlite", "fittrack.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Initialize Tables
	db.Exec(`CREATE TABLE IF NOT EXISTS workout_category (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT UNIQUE);`)
	db.Exec(`CREATE TABLE IF NOT EXISTS daily_routine (id INTEGER PRIMARY KEY AUTOINCREMENT, workout_date TEXT, category TEXT, duration INTEGER, meal TEXT);`)

	// Routes
	http.HandleFunc("/", handleDashboard)
	http.HandleFunc("/save", handleSaveRoutine)
	http.HandleFunc("/delete-history", handleDeleteHistory)
	http.HandleFunc("/workouts", handleWorkouts)
	http.HandleFunc("/workouts/save", handleSaveCategory)
	http.HandleFunc("/workouts/delete", handleDeleteCategory)
	http.HandleFunc("/progress", handleProgress)

	fmt.Println("🚀 Fit-Track running at http://0.0.0.0:3000")
	log.Fatal(http.ListenAndServe(":3000", nil))
}

func getProgressReport() ProgressReport {
	stats := getStats()
	score := 0
	if stats.TotalWorkouts >= 4 {
		score += 40
	} else {
		score += (stats.TotalWorkouts * 10)
	}
	if stats.TotalMinutes >= 180 {
		score += 30
	} else {
		score += (stats.TotalMinutes / 6)
	}

	rows, _ := db.Query("SELECT meal FROM daily_routine WHERE date(workout_date) >= date('now', '-7 days')")
	defer rows.Close()
	healthy, total := 0, 0
	for rows.Next() {
		var m string
		rows.Scan(&m)
		total++
		m = strings.ToLower(m)
		if strings.Contains(m, "rice") || strings.Contains(m, "apple") || strings.Contains(m, "chicken") {
			healthy++
		}
	}
	foodStatus := "Unbalanced"
	if total > 0 {
		ratio := float64(healthy) / float64(total)
		score += int(ratio * 30)
		if ratio > 0.6 {
			foodStatus = "Healthy"
		}
	}
	lvl := "Active"
	if score > 80 {
		lvl = "Elite"
	} else if score < 40 {
		lvl = "Beginner"
	}
	return ProgressReport{Score: score, Level: lvl, FoodStatus: foodStatus, Consistency: (stats.TotalWorkouts * 100) / 4}
}

// Handlers
func handleDashboard(w http.ResponseWriter, r *http.Request) {
	rows, _ := db.Query("SELECT name FROM workout_category")
	var names []string
	for rows.Next() {
		var n string
		rows.Scan(&n)
		names = append(names, n)
	}
	rows.Close()

	hist, _ := db.Query("SELECT id, workout_date, category, duration, meal FROM daily_routine ORDER BY date(workout_date) DESC")
	var history []RoutineRecord
	for hist.Next() {
		var rec RoutineRecord
		hist.Scan(&rec.ID, &rec.Date, &rec.Category, &rec.Duration, &rec.Meal)
		history = append(history, rec)
	}
	hist.Close()
	template.Must(template.ParseFiles("index.html")).Execute(w, PageData{CatNames: names, History: history, Stats: getStats()})
}

func handleProgress(w http.ResponseWriter, r *http.Request) {
	template.Must(template.ParseFiles("progress.html")).Execute(w, PageData{Stats: getStats(), Progress: getProgressReport()})
}

func handleWorkouts(w http.ResponseWriter, r *http.Request) {
	rows, _ := db.Query("SELECT id, name FROM workout_category")
	var cats []WorkoutCategory
	for rows.Next() {
		var c WorkoutCategory
		rows.Scan(&c.ID, &c.Name)
		cats = append(cats, c)
	}
	rows.Close()
	template.Must(template.ParseFiles("workouts.html")).Execute(w, PageData{Categories: cats})
}

func handleSaveRoutine(w http.ResponseWriter, r *http.Request) {
	id := r.FormValue("id")
	if id == "" {
		db.Exec(`INSERT INTO daily_routine (workout_date, category, duration, meal) VALUES (?, ?, ?, ?)`,
			r.FormValue("date"), r.FormValue("category"), r.FormValue("duration"), r.FormValue("meal"))
	} else {
		db.Exec(`UPDATE daily_routine SET workout_date=?, category=?, duration=?, meal=? WHERE id=?`,
			r.FormValue("date"), r.FormValue("category"), r.FormValue("duration"), r.FormValue("meal"), id)
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func handleDeleteHistory(w http.ResponseWriter, r *http.Request) {
	db.Exec("DELETE FROM daily_routine WHERE id = ?", r.URL.Query().Get("id"))
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func handleSaveCategory(w http.ResponseWriter, r *http.Request) {
	id, name := r.FormValue("id"), r.FormValue("name")
	if id == "" {
		db.Exec("INSERT INTO workout_category (name) VALUES (?)", name)
	} else {
		db.Exec("UPDATE workout_category SET name = ? WHERE id = ?", name, id)
	}
	http.Redirect(w, r, "/workouts", http.StatusSeeOther)
}

func handleDeleteCategory(w http.ResponseWriter, r *http.Request) {
	db.Exec("DELETE FROM workout_category WHERE id = ?", r.URL.Query().Get("id"))
	http.Redirect(w, r, "/workouts", http.StatusSeeOther)
}
