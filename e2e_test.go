package main

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/tebeka/selenium"
)

const (
	chromeDriverPath = "/opt/homebrew/bin/chromedriver"
	port             = 4444
	baseURL          = "http://localhost:8080/bouquiniste"
)

func TestE2EBooks(t *testing.T) {
	opts := []selenium.ServiceOption{}
	service, err := selenium.NewChromeDriverService(chromeDriverPath, port, opts...)
	if err != nil {
		t.Fatalf("Ошибка запуска ChromeDriver: %v", err)
	}
	defer service.Stop()

	caps := selenium.Capabilities{"browserName": "chrome"}
	driver, err := selenium.NewRemote(caps, fmt.Sprintf("http://localhost:%d/wd/hub", port))
	if err != nil {
		t.Fatalf("Ошибка открытия сессии WebDriver: %s", err)
	}
	defer driver.Quit()

	// 1️⃣ Открываем страницу букиниста
	if err := driver.Get(baseURL); err != nil {
		t.Fatalf("Ошибка загрузки страницы: %s", err)
	}
	time.Sleep(3 * time.Second)

	// 2️⃣ Открываем список книг
	viewBooksButton, err := driver.FindElement(selenium.ByID, "read-books-btn")
	if err != nil {
		t.Fatalf("Не найдена кнопка 'View All Books': %s", err)
	}
	viewBooksButton.Click()
	time.Sleep(3 * time.Second)

	// 3️⃣ **Фильтрация книг по названию**
	filterTitleInput, err := driver.FindElement(selenium.ByID, "filter-title")
	if err != nil {
		t.Fatalf("Не найдено поле ввода title: %s", err)
	}
	filterTitleInput.SendKeys("hp")

	sortByDropdown, err := driver.FindElement(selenium.ByID, "sort-by")
	if err != nil {
		t.Fatalf("Не найден dropdown 'Sort by': %s", err)
	}
	sortByDropdown.SendKeys("Title")

	applyFiltersButton, err := driver.FindElement(selenium.ByID, "apply-filters")
	if err != nil {
		t.Fatalf("Не найдена кнопка 'Apply': %s", err)
	}
	applyFiltersButton.Click()
	time.Sleep(5 * time.Second) // Ждём обновления таблицы

	// 4️⃣ **Проверяем, что книга найдена**
	booksTable, err := driver.FindElement(selenium.ByID, "books-table")
	if err != nil {
		t.Fatalf("Не найдена таблица с книгами: %s", err)
	}

	booksText, _ := booksTable.Text()
	fmt.Println("📚 Текст в таблице:", booksText) // Проверяем, что в таблице что-то есть

	// Проверяем, есть ли "hp" в текстовом представлении таблицы
	if !containsIgnoreCase(booksText, "hp") {
		t.Errorf("Книга не найдена в таблице! Ожидалась 'hp'. Таблица содержит: %s", booksText)
	}
}

// **Функция поиска текста (без учета регистра)**
func containsIgnoreCase(source, substr string) bool {
	return strings.Contains(strings.ToLower(source), strings.ToLower(substr))
}


