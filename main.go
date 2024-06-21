package main

import (
	"database/sql"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/joho/godotenv"
	"github.com/tealeg/xlsx"
	"github.com/tidwall/gjson"
	"log"
	"os"
	"path/filepath"
	"strconv"
)

type Storage struct {
	Conn *sql.DB
}

func New(dsn string) (*Storage, error) {
	// Подключение к базе данных
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	// Проверка соединения
	if err := db.Ping(); err != nil {
		return nil, err
	}

	return &Storage{Conn: db}, nil
}

func main() {
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Fatal(err)
	}

	err = godotenv.Load(filepath.Join(dir, ".env"))
	if err != nil {
		log.Fatalf("Some error occured. Err: %s", err)
	}

	// Параметры подключения к базе данных
	dsn := os.Getenv("DNS_DB")

	db, err := New(dsn)
	if err != nil {
		log.Fatalf("Could not connect to database: %v", err)
	}

	defer db.Conn.Close()

	fmt.Printf("TELEGRAM BOT TOKEN: %s", os.Getenv("TELEGRAM_BOT_TOKEN"))

	bot, err := tgbotapi.NewBotAPI(os.Getenv("TELEGRAM_BOT_TOKEN"))
	if err != nil {
		log.Panic(err)
	}

	bot.Debug = true

	log.Printf("Authorized on account %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := bot.GetUpdatesChan(u)

	var msg tgbotapi.MessageConfig

	for update := range updates {

		if update.Message == nil {
			continue
		}

		if update.Message.IsCommand() {
			handleCommand(bot, update.Message)
			continue
		}

		if update.Message.Text == "Download" {

			fileName, err := db.getUserCoins()
			if err != nil {
				log.Fatalf("Ошибка выполнения запроса: %v", err)
			}

			file := tgbotapi.NewDocumentUpload(update.Message.Chat.ID, fileName)
			file.ReplyMarkup = getMainKeyboard()
			bot.Send(file)
		} else {
			msg = tgbotapi.NewMessage(update.Message.Chat.ID, "Hello! Choose an action:")
			msg.ReplyMarkup = getMainKeyboard()
			bot.Send(msg)
		}

	}

	fmt.Println("Bot ready!")
}

func (db *Storage) getUserCoins() (string, error) {
	// SQL запрос к базе данных
	query := "select `users`.`name`, `users`.`last_name`, `users`.`parent_name`, `users`.`username`, `users`.`email`, `users`.`coins`, CONCAT(groups.title) as group_title, `teacher`.`name` as `teacher_name` from `users` left join `group_user` on `group_user`.`user_id` = `users`.`id` left join `groups` on `groups`.`id` = `group_user`.`group_id` left join `admins` as `teacher` on `teacher`.`id` = `groups`.`teacher_id` where `users`.`school_id` = 1231 group by `users`.`id`"
	rows, err := db.Conn.Query(query)
	if err != nil {
		log.Fatal(err)
		return "", err
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			log.Fatal(err)
		}
	}(rows)

	// Получение колонок
	columns, err := rows.Columns()

	if err != nil {
		log.Fatal(err)
		return "", err
	}

	// Создание нового файла Excel
	file := xlsx.NewFile()
	sheet, err := file.AddSheet("Sheet1")
	if err != nil {
		log.Fatal(err)
		return "", err
	}

	// Запись заголовков в файл Excel
	headerRow := sheet.AddRow()
	for _, colName := range columns {
		cell := headerRow.AddCell()
		cell.Value = colName
	}

	// Запись данных в файл Excel
	for rows.Next() {
		values := make([]sql.RawBytes, len(columns))
		scanArgs := make([]interface{}, len(values))

		for i := range values {
			scanArgs[i] = &values[i]
		}

		err = rows.Scan(scanArgs...)
		if err != nil {
			log.Fatal(err)
			return "", err
		}

		dataRow := sheet.AddRow()

		for key, value := range values {

			// fmt.Println(key, ":", string(value))

			if key == 5 && string(value) != "" {
				//	fmt.Println(string(value))

				result := gjson.Parse(string(value))
				//	fmt.Println(result)

				// Проходимся по элементам массива

				totalSumma := 0
				result.ForEach(func(key, value gjson.Result) bool {
					if value.IsObject() {
						//fmt.Println("Object:")
						value.ForEach(func(k, v gjson.Result) bool {
							//fmt.Printf("  %s: %v\n", k.String(), v.Value())
							if v.Type == gjson.Number {
								val := int(v.Int())
								totalSumma += val
							} else {
								fmt.Println("Value is not a number")
							}

							return true // продолжить итерацию
						})
					} else {
						fmt.Printf("Unknown type: %s\n", value.Type.String())
					}
					return true // продолжить итерацию
				})

				cell := dataRow.AddCell()
				cell.Value = strconv.Itoa(totalSumma)
			} else {
				cell := dataRow.AddCell()
				cell.Value = string(value)
			}

		}
	}

	// Проверка на ошибки при итерации по строкам
	if err = rows.Err(); err != nil {
		log.Fatal(err)
		return "", err
	}

	fileName := "upload/user_coin_list.xlsx"

	// Сохранение файла Excel
	err = file.Save(fileName)

	if err != nil {
		log.Fatal(err)
		return "", err
	}

	fmt.Println("Данные успешно сохранены !" + fileName)

	return fileName, nil
}

func handleCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	switch message.Command() {
	case "start":
		msg := tgbotapi.NewMessage(message.Chat.ID, "Welcome!.")
		msg.ReplyMarkup = getMainKeyboard()
		bot.Send(msg)
	default:
		msg := tgbotapi.NewMessage(message.Chat.ID, "I don't know that command.")
		bot.Send(msg)
	}
}

func getMainKeyboard() tgbotapi.ReplyKeyboardMarkup {
	button1 := tgbotapi.NewKeyboardButton("Download")
	row1 := tgbotapi.NewKeyboardButtonRow(button1)

	keyboard := tgbotapi.NewReplyKeyboard(row1)
	return keyboard
}
