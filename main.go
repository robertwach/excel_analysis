package main

import (
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"

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

		docRows, err := ReadExelFile(dest)
		if err != nil {
			c.HTML(http.StatusTemporaryRedirect, "error.html", gin.H{
				"error": err.Error(),
			})
			return
		}

		wg := sync.WaitGroup{}

		wg.Add(1)
		var matchErrors error
		phraseErrors := ""
		var msg string
		var rows string
		go func() {
			defer wg.Done()

			r, m, err := DeepCheck(docRows)

			rows = r
			msg = m
			matchErrors = err

		}()

		if matchErrors != nil {
			if err != nil {
				c.HTML(http.StatusTemporaryRedirect, "error.html", gin.H{
					"error": matchErrors.Error(),
				})
			}
		}

		wg.Add(1)
		func() {
			defer wg.Done()
			_, pe := PhraseCheck(docRows)

			if pe != nil {
				phraseErrors = pe.Error()
			}

		}()

		wg.Wait()

		c.HTML(
			http.StatusOK,
			"results.html",
			gin.H{
				"title":        "Check results",
				"rows":         template.HTML(rows),
				"msg":          template.HTML(msg),
				"phraseErrors": template.HTML(phraseErrors),
			},
		)
	})

	r.Run()
}
func DoDocCheck(rows [][]string) (string, string, error) {
	deepCheck := [][]string{}

	numberOfColumns := len(rows[4])

	if numberOfColumns < 3 {
		return "", "This file has less than 2 columns", errors.New("this file has less than 2 columns")
	}

	for i, row := range rows {
		lineLens := []int{}
		// cell_num := ""
		if i > 0 {

			ln := len(row)

			if ln == 1 {
				continue
			}

			for j, colCell := range row {

				if j == 0 {
					continue
				} else if j > 0 {

					v := strings.Split(strings.ReplaceAll(colCell, "\r\n", "\n"), "\n")
					if len(v) == 1 {
						_, err := strconv.ParseInt(colCell, 10, 64)
						if err == nil {
							continue
						}
					}
					l := len(v)
					lineLens = append(lineLens, l)
				}

			}
		}
		if len(lineLens) > 0 {

			primaryValue := lineLens[0]
			for _, item := range lineLens {
				if item != primaryValue {
					deepCheck = append(deepCheck, row)
					break
				}
			}
		}
	}

	if len(deepCheck) == 0 {
		fmt.Println("Check complete => all cells OK")
		return "", "No errors found", nil
	}

	fmt.Println("Deep check >>>>>>>>>>>>>>", len(deepCheck))
	return DeepCheck(deepCheck)
}
func ReadExelFile(fileName string) ([][]string, error) {
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
	return f.GetRows(sheets[0])
}

func DeepCheck(rows [][]string) (string, string, error) {
	erroredRows := ""
	str := ""
	for _, row := range rows {
		cell_num := ""

		lineLens := []int{}

		for j, colCell := range row {
			if j == 0 {
				cell_num = colCell
			} else {
				v := strings.Split(strings.ReplaceAll(colCell, "\r\n", "\n"), "\n")
				if len(v) == 1 {
					_, err := strconv.ParseInt(colCell, 10, 64)
					if err == nil {
						continue
					}
				}
				v2 := []string{}
				for _, item := range v {
					if strings.Trim(item, " ") != "" {
						v2 = append(v2, item)
					}
				}
				lineLens = append(lineLens, len(v2))
			}
			// if i < 20 {
			// 	fmt.Println(cell_num, lineLens)
			// }

		}

		if len(lineLens) > 0 {
			th := "<th> Number </th>"
			for i := 0; i < len(lineLens); i++ {
				th += fmt.Sprintf("<th> Column %d </th>", i+1)
			}
			if erroredRows == "" {
				erroredRows += fmt.Sprintf("<tr class='w-full bg-pink-500 text-white'> %s </tr>", th)
			}

			primaryValue := lineLens[0]
			for _, item := range lineLens {
				if item != primaryValue {
					// fmt.Println("line lens >>>>>>", lineLens)

					lensString := strings.Trim(strings.Replace(fmt.Sprint(lineLens), " ", ", ", -1), "[]")

					str += fmt.Sprintf(" <li class='border border-blue-300 p-2 rounded'>Comfirm the validity of  row %v. <br /> Items mismatch <i>(item count per cell)</i> %s </li>", cell_num, lensString)
					s := ""

					for _, item := range row {
						s += fmt.Sprintf("<td> <pre> %s </pre> </td>", item)
					}

					erroredRows += fmt.Sprintf("<tr> %s </tr>", s)
					break
				}
			}
		}

	}

	return erroredRows, str, nil
}

func PhraseCheck(rows [][]string) (bool, error) {
	specialChars := []string{"#", "{", "["}

	phraseErrors := []string{}

	for _, row := range rows {
		cell_num := ""

		activeRows := [][]string{}

		for j, colCell := range row {
			if j == 0 {
				cell_num = colCell
			} else {
				v := strings.Split(strings.ReplaceAll(colCell, "\r\n", "\n"), "\n")
				if len(v) == 1 {
					_, err := strconv.ParseInt(colCell, 10, 64)
					if err == nil {
						continue
					}
				}
				v2 := []string{}
				for _, item := range v {
					if strings.Trim(item, " ") != "" {
						v2 = append(v2, item)
					}
				}
				activeRows = append(activeRows, v2)

			}
		}

		rowLength := 0

		if len(activeRows) > 0 {
			rowLength = len(activeRows[0])
		}

		for i := 0; i < rowLength; i++ {
			// do CharacterMatches for each row at index i for every special character
			stringsToCheck := []string{}

			for _, row := range activeRows {
				stc := ""
				if len(row) > i {
					stc = row[i]
				}
				stringsToCheck = append(stringsToCheck, stc)
			}

			for _, char := range specialChars {
				_, err := CharacterMatches(stringsToCheck, char)
				if err != nil {
					phraseErrors = append(phraseErrors, fmt.Sprintf("<li class='border-b border-white pb-2'>Row %s error: <span> %s </span> </li>", cell_num, err.Error()))
				}
			}

		}
	}

	return len(phraseErrors) == 0, fmt.Errorf(strings.Join(phraseErrors, "\n"))
}

func CharacterMatches(strs []string, specialChar string) (bool, error) {
	hasSpecialChar := false
	for _, str := range strs {
		if strings.Contains(str, specialChar) {
			hasSpecialChar = true
			break
		}
	}

	if !hasSpecialChar {
		return true, nil
	}
	errStr := ""
	// var r regexp.Regexp

	var re *regexp.Regexp
	closeBracket := "]"
	if specialChar == "[" || specialChar == "{" {
		if specialChar == "{" {
			closeBracket = "}"
			re = regexp.MustCompile(fmt.Sprintf("(?m)%s(.+?)%s", specialChar, closeBracket))
		} else {
			// fmt.Println("creating regest for []")
			re = regexp.MustCompile(`\[([^[\]]*)\]`)
		}
	} else {
		closeBracket = "#"
		re = regexp.MustCompile(`(?m)#(.+?)#`)
	}

	lengths := map[int][]string{}

	for i, str := range strs {
		matches := re.FindAllString(str, -1)
		lengths[i] = matches
	}

	primaryLen := len(lengths[0])
	lengthsMatch := true

	for _, matches := range lengths {
		if len(matches) != primaryLen {
			lengthsMatch = false
			break
		}
	}
	class := "text-red-500"
	if specialChar == "#" {
		class = "text-amber-500"
	} else if specialChar == "{" {
		class = "text-purple-600"
	}

	if !lengthsMatch {

		return false, fmt.Errorf("<b><span class='%s'>%s<span class='%s'>phrase<span>%s</span> </b> pattern mismatch", class, specialChar, class, closeBracket)
	}

	if specialChar == "[" {
		return true, nil
	}

	primaryMatches := lengths[0]

	for i := 0; i < len(primaryMatches); i++ {
		for j := 0; j < len(lengths); j++ {
			if j == 0 {
				continue
			}
			if primaryMatches[i] != lengths[j][i] {
				//  return false, fmt.Errorf("phrases do not match: %s != %s", primaryMatches[i], lengths[j][i])
				errStr += fmt.Sprintf("<li class='w-ful'> phrases do not match: %s != %s </li>", primaryMatches[i], lengths[j][i])
			}
		}
	}

	if errStr != "" {
		return false, errors.New(errStr)
	}

	return true, nil

}
