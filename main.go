package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
)

func main() {
	r := gin.Default()
	r.LoadHTMLGlob("templates/*")
	r.MaxMultipartMemory = 8 << 20 // 8 MiB

	r.GET("/", func(c *gin.Context) {
		c.HTML(
			http.StatusOK,
			"index.html",
			gin.H{
				"title": "Check your files",
			},
		)
	})

	r.POST("/", func(c *gin.Context) {
		file, err := c.FormFile("file")

		if err != nil {
			c.HTML(http.StatusTemporaryRedirect, "error.html", gin.H{
				"error": err.Error(),
			})
			return
		}
		log.Println(file.Filename)

		dest := "./public/abc.xlsx"

		err = c.SaveUploadedFile(file, dest)

		if err != nil {
			c.HTML(http.StatusTemporaryRedirect, "error.html", gin.H{
				"error": err.Error(),
			})
			return
		}

		rows, msg, err := ReadExelFile(dest)

		if err != nil {
			// c.HTML(
			// 	http.StatusOK,
			// 	"index.html",
			// 	gin.H{
			// 		"title": "Check your files",
			// 	},
			// )
			c.HTML(http.StatusTemporaryRedirect, "error.html", gin.H{
				"error": err.Error(),
			})
		}

		c.HTML(
			http.StatusOK,
			"results.html",
			gin.H{
				"title": "Check results",
				"rows":  template.HTML(rows),
				"msg":   template.HTML(msg),
			},
		)
	})

	r.Run()

}

func ReadExelFile(fileName string) (string, string, error) {
	f, err := excelize.OpenFile(fileName)

	if err != nil {
		panic(err)
	}

	defer func() {
		if err := f.Close(); err != nil {
			panic(err)
		}
	}()
	sheets := f.GetSheetList()
	rows, err := f.GetRows(sheets[0])
	if err != nil {
		fmt.Println(err)
		return "", "", err
	}
	deepCheck := [][]string{}
	// deepCheck := [][]string{}

	for i, row := range rows {
		cell_1_lines := 0
		cell_2_lines := 0
		// cell_num := ""
		if i > 3 {

			ln := len(row)

			if ln == 1 {
				continue
			}

			for j, colCell := range row {
				// if j == 0 {
				// 	//  fmt.Println("cell number is => ", colCell)
				// 	cell_num = colCell
				// }
				if j == 1 {
					v := strings.Split(strings.ReplaceAll(colCell, "\r\n", "\n"), "\n")
					cell_1_lines = len(v)
				}

				if j == 2 {
					v := strings.Split(strings.ReplaceAll(colCell, "\r\n", "\n"), "\n")
					cell_2_lines = len(v)

				}
			}
		}

		// fmt.Println(cell_num, cell_1_lines, cell_2_lines)
		if cell_1_lines != cell_2_lines {
			// fmt.Println("====> This row has unequal lines", cell_num)
			deepCheck = append(deepCheck, row)
		}
		// fmt.Println()
	}

	if len(deepCheck) == 0 {
		fmt.Println("Check complete => all cells OK")
		return "", "No errors found", nil
	}

	fmt.Println("Deep check", len(deepCheck))
	return DeepCheck(deepCheck)
}

func DeepCheck(rows [][]string) (string, string, error) {
	// erroredRows := [][]string{}
	erroredRows := ""
	str := ""
	for _, row := range rows {
		cell_1_lines := 0
		cell_2_lines := 0
		cell_1_data := ""
		cell_2_data := ""
		cell_num := ""

		for j, colCell := range row {
			if j == 0 {
				//  fmt.Println("cell number is => ", colCell)
				cell_num = colCell
			}
			if j == 1 {
				v := strings.Split(strings.ReplaceAll(colCell, "\r\n", "\n"), "\n")
				// fliter empty lines
				v2 := []string{}
				cell_1_data = colCell
				for _, item := range v {
					if strings.Trim(item, " ") != "" {
						// fmt.Println(k, " appending line item ====>", item)
						v2 = append(v2, item)
					}
				}
				cell_1_lines = len(v2)
			}

			if j == 2 {
				v := strings.Split(strings.ReplaceAll(colCell, "\r\n", "\n"), "\n")
				cell_2_data = colCell
				// fliter empty lines
				v2 := []string{}
				for _, item := range v {
					if strings.Trim(item, " ") != "" {
						// fmt.Println(k, " appending line item ====>", item)
						v2 = append(v2, item)
					}

				}
				cell_2_lines = len(v2)

			}
		}

		if cell_1_lines != cell_2_lines {
			str += fmt.Sprintf("<li>Comfirm the validity of  row %v  </li>", cell_num)
			// string with cell 1 data and cell 2 data
			s := fmt.Sprintf("<tr> <td> <pre> %s </pre> </td> <td> <pre> %s </pre> </td> <td> <pre> %s </pre> </td> </tr>", cell_num, cell_1_data, cell_2_data)

			// erroredRows = append(erroredRows, s)
			erroredRows += s
		}
	}

	return erroredRows, str, nil
}
