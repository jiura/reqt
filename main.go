/* TODO:

- Stopwatch or progress bar when submitting request
- Exec to open Body input on editor of choice
- Reorganize code in more than one file

*/

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
)

var (
	DEBUG bool

	STYLE_NONE    = lipgloss.NewStyle()
	STYLE_FOCUSED = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	STYLE_BLURRED = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	CURSOR_STYLE           = STYLE_FOCUSED
	HELP_STYLE             = STYLE_BLURRED
	CURSOR_MODE_HELP_STYLE = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))

	BLURRED_LABEL_METHOD = fmt.Sprintf("\n%s\n", STYLE_BLURRED.Render("METHOD:"))
	FOCUSED_LABEL_METHOD = fmt.Sprintf("\n%s\n", STYLE_FOCUSED.Render("METHOD:"))

	BLURRED_LABEL_URL = fmt.Sprintf("\n\n%s", STYLE_BLURRED.Render("URL: "))
	FOCUSED_LABEL_URL = fmt.Sprintf("\n\n%s", STYLE_FOCUSED.Render("URL: "))

	BLURRED_LABEL_BODY = fmt.Sprintf("\n\n%s\n", STYLE_BLURRED.Render("BODY:"))
	FOCUSED_LABEL_BODY = fmt.Sprintf("\n\n%s\n", STYLE_FOCUSED.Render("BODY:"))

	BLURRED_LABEL_HEADERS = fmt.Sprintf("\n\n%s", STYLE_BLURRED.Render("HEADERS:"))
	FOCUSED_LABEL_HEADERS = fmt.Sprintf("\n\n%s", STYLE_FOCUSED.Render("HEADERS:"))

	BLURRED_BTN_SUBMIT = fmt.Sprintf("[ %s ]", STYLE_BLURRED.Render("Submit"))
	FOCUSED_BTN_SUBMIT = STYLE_FOCUSED.Render("[ Submit ]")
)

// enum form items
const (
	METHOD_FORM_ITEM_ID uint8 = iota
	URL_FORM_ITEM_ID
	BODY_FORM_ITEM_ID
	SUBMIT_FORM_ITEM_ID = 255
)

type FormItem struct {
	Id uint8
}

type FormTextInput struct {
	FormItem

	TextInput textinput.Model
}

type FormAreaInput struct {
	FormItem

	TextArea textarea.Model
}

type FormRadio struct {
	FormItem

	SelectedIndex uint8
	Choices       []string
	Len           uint8
}

func (fr *FormRadio) NewRadio(id uint8, choices []string) {
	fr.Id = id
	fr.Choices = choices
	fr.Len = uint8(len(choices))
}

type HttpHeader struct {
	Name  FormTextInput
	Value FormTextInput
}

type model struct {
	// General configs
	cursorMode cursor.Mode

	// Form state
	currentItemIndex uint8

	methods FormRadio

	urlInput          FormTextInput
	bodyInput         FormAreaInput
	urlAndBodyMenuLen uint8

	headers []HttpHeader

	responseInfo string
	responseBody string

	currentWidth int // NOTE: This is only relevant to wrap response body text
}

func initialModel() model {
	m := model{
		headers: make([]HttpHeader, 0, 2),
	}

	m.methods.NewRadio(METHOD_FORM_ITEM_ID, []string{"GET", "POST", "PUT", "PATCH", "DELETE"})

	m.urlInput.Id = URL_FORM_ITEM_ID

	{ // URL text input
		m.urlInput.TextInput = textinput.New()
		m.urlInput.TextInput.Cursor.Style = CURSOR_STYLE
		m.urlInput.TextInput.CharLimit = 128

		// TODO: This isn't working. Find out why.
		// m.urlInput.TextInput.Placeholder = "https://example.com"
	}

	m.bodyInput.Id = BODY_FORM_ITEM_ID

	{ // Body text area
		m.bodyInput.TextArea = textarea.New()
		m.bodyInput.TextArea.Cursor.Style = CURSOR_STYLE
		m.bodyInput.TextArea.Placeholder = "{\n  \"foo\":\"bar\"\n}"
		m.bodyInput.TextArea.ShowLineNumbers = false
	}

	return m
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m *model) submitRequest() {
	var err error

	var method string = m.methods.Choices[m.methods.SelectedIndex]
	var url string = m.urlInput.TextInput.Value()

	if url == "" {
		m.responseInfo = "No URL found.\n"
		return
	}

	if !strings.Contains(url, "//") {
		url = "https://" + url
	}

	var bodyJson []byte
	bodyJson, err = json.Marshal(m.bodyInput.TextArea.Value())
	if err != nil {
		m.responseInfo = err.Error()
		return
	}

	var req *http.Request
	req, err = http.NewRequest(method, url, bytes.NewReader(bodyJson))
	if err != nil {
		m.responseInfo = err.Error()
		return
	}

	for i := 0; i < len(m.headers); i++ {
		var name string = m.headers[i].Name.TextInput.Value()
		var value string = m.headers[i].Value.TextInput.Value()
		req.Header.Set(name, value)
	}

	var resp *http.Response
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		m.responseInfo = err.Error()
		return
	}
	defer resp.Body.Close()

	sb := strings.Builder{}

	sb.WriteString(resp.Status)
	sb.WriteString("\n\n--------------------\n")

	for name := range resp.Header {
		sb.WriteString("\n")
		sb.WriteString(name)
		sb.WriteString(": ")
		sb.WriteString(resp.Header.Get(name))
	}
	sb.WriteString("\n\n--------------------\n\n")

	var respBodyBytes []byte
	respBodyBytes, err = io.ReadAll(resp.Body)
	if err != nil {
		sb.WriteString(err.Error())
	}

	m.responseInfo = sb.String()
	m.responseBody = string(respBodyBytes)
}

func (m *model) updateUrlTextInput(msg tea.Msg) tea.Cmd {
	cmd := func() tea.Msg {
		return ""
	}

	m.urlInput.TextInput, cmd = m.urlInput.TextInput.Update(msg)

	return cmd
}

func (m *model) updateBodyTextArea(msg tea.Msg) tea.Cmd {
	cmd := func() tea.Msg {
		return ""
	}

	m.bodyInput.TextArea, cmd = m.bodyInput.TextArea.Update(msg)

	return cmd
}

func (m *model) updateCurrentInput(msg tea.Msg) tea.Cmd {
	cmd := func() tea.Msg {
		return ""
	}

	switch m.currentItemIndex {
	case URL_FORM_ITEM_ID:
		m.urlInput.TextInput, cmd = m.urlInput.TextInput.Update(msg)

	case BODY_FORM_ITEM_ID:
		m.bodyInput.TextArea, cmd = m.bodyInput.TextArea.Update(msg)

	default:
		if len(m.headers) > 0 &&
			m.currentItemIndex > BODY_FORM_ITEM_ID &&
			m.currentItemIndex < SUBMIT_FORM_ITEM_ID {

			for i := range m.headers {
				if m.headers[i].Name.Id == m.currentItemIndex {
					m.headers[i].Name.TextInput, cmd = m.headers[i].Name.TextInput.Update(msg)
					break
				} else if m.headers[i].Value.Id == m.currentItemIndex {
					m.headers[i].Value.TextInput, cmd = m.headers[i].Value.TextInput.Update(msg)
					break
				}
			}
		}
	}

	return cmd
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	currentNumOfItems := uint8(len(m.headers)*2 + 3)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, tea.Quit

		case "ctrl+s":
			m.submitRequest()
			m.currentWidth, _, _ = term.GetSize(int(os.Stdout.Fd()))
			return m, tea.Quit

		case "enter":
			switch m.currentItemIndex {
			case METHOD_FORM_ITEM_ID:
				m.currentItemIndex = URL_FORM_ITEM_ID

			case URL_FORM_ITEM_ID:
				m.currentItemIndex = BODY_FORM_ITEM_ID

			case SUBMIT_FORM_ITEM_ID:
				m.submitRequest()
				m.currentWidth, _, _ = term.GetSize(int(os.Stdout.Fd()))
				return m, tea.Quit

			case BODY_FORM_ITEM_ID:
				return m, m.updateBodyTextArea(msg)

			default:
				if m.currentItemIndex < currentNumOfItems-1 {
					m.currentItemIndex++
				} else { // Last item
					m.currentItemIndex = SUBMIT_FORM_ITEM_ID
				}
			}

		case "up":
			switch m.currentItemIndex {
			case METHOD_FORM_ITEM_ID:
				break

			case URL_FORM_ITEM_ID:
				m.currentItemIndex = METHOD_FORM_ITEM_ID

			case BODY_FORM_ITEM_ID:
				if m.bodyInput.TextArea.Line() != 0 {
					return m, m.updateBodyTextArea(msg)
				} else {
					m.currentItemIndex = URL_FORM_ITEM_ID
				}

			case SUBMIT_FORM_ITEM_ID:
				if len(m.headers) > 0 {
					m.currentItemIndex = m.headers[len(m.headers)-1].Name.Id
				} else {
					m.currentItemIndex = BODY_FORM_ITEM_ID
				}

			default:
				if m.currentItemIndex <= BODY_FORM_ITEM_ID+2 { // First header value
					m.currentItemIndex = BODY_FORM_ITEM_ID
				} else {
					m.currentItemIndex -= 2
				}
			}

		case "down":
			switch m.currentItemIndex {
			case METHOD_FORM_ITEM_ID:
				m.currentItemIndex = URL_FORM_ITEM_ID

			case URL_FORM_ITEM_ID:
				m.currentItemIndex = BODY_FORM_ITEM_ID

			case BODY_FORM_ITEM_ID:
				if m.bodyInput.TextArea.Line() != m.bodyInput.TextArea.LineCount()-1 {
					return m, m.updateBodyTextArea(msg)
				} else {
					if len(m.headers) > 0 {
						m.currentItemIndex = m.headers[0].Name.Id
					} else {
						m.currentItemIndex = SUBMIT_FORM_ITEM_ID
					}
				}

			case SUBMIT_FORM_ITEM_ID:
				break

			default:
				if m.currentItemIndex < currentNumOfItems-1 {
					m.currentItemIndex++
				} else { // Last item
					m.currentItemIndex = SUBMIT_FORM_ITEM_ID
				}
			}

		case "left":
			switch m.currentItemIndex {
			case METHOD_FORM_ITEM_ID:
				if m.methods.SelectedIndex > 0 {
					m.methods.SelectedIndex--
				}

			case URL_FORM_ITEM_ID:
				return m, m.updateUrlTextInput(msg)

			case BODY_FORM_ITEM_ID:
				return m, m.updateBodyTextArea(msg)

			case SUBMIT_FORM_ITEM_ID:
				break

			default:
				if m.currentItemIndex > BODY_FORM_ITEM_ID {
					m.currentItemIndex--
				}
			}

		case "right":
			switch m.currentItemIndex {
			case METHOD_FORM_ITEM_ID:
				if m.methods.SelectedIndex < m.methods.Len-1 {
					m.methods.SelectedIndex++
				}

			case URL_FORM_ITEM_ID:
				return m, m.updateUrlTextInput(msg)

			case BODY_FORM_ITEM_ID:
				return m, m.updateBodyTextArea(msg)

			case SUBMIT_FORM_ITEM_ID:
				break

			default:
				if m.currentItemIndex < currentNumOfItems-1 {
					m.currentItemIndex++
				} else { // Last item
					m.currentItemIndex = SUBMIT_FORM_ITEM_ID
				}
			}

		case "tab":
			switch m.currentItemIndex {
			case BODY_FORM_ITEM_ID:
				m.bodyInput.TextArea.InsertString("  ")
			}

		case "ctrl+up":
			switch m.currentItemIndex {
			case BODY_FORM_ITEM_ID:
				m.currentItemIndex = URL_FORM_ITEM_ID
			}

		case "ctrl+down":
			switch m.currentItemIndex {
			case BODY_FORM_ITEM_ID:
				if len(m.headers) > 0 {
					m.currentItemIndex = m.headers[0].Name.Id
				} else {
					m.currentItemIndex = SUBMIT_FORM_ITEM_ID
				}
			}

		default: // Not a command, process text input
			return m, m.updateCurrentInput(msg)
		}
	}

	// Handle each input state
	cmds := make([]tea.Cmd, currentNumOfItems)

	if m.currentItemIndex != URL_FORM_ITEM_ID {
		m.urlInput.TextInput.Blur()
		m.urlInput.TextInput.PromptStyle = STYLE_NONE
		m.urlInput.TextInput.TextStyle = STYLE_NONE
	} else {
		cmds[URL_FORM_ITEM_ID] = m.urlInput.TextInput.Focus()
		m.urlInput.TextInput.PromptStyle = STYLE_FOCUSED
		m.urlInput.TextInput.TextStyle = STYLE_FOCUSED
	}

	if m.currentItemIndex != BODY_FORM_ITEM_ID {
		m.bodyInput.TextArea.Blur()
	} else {
		cmds[BODY_FORM_ITEM_ID] = m.bodyInput.TextArea.Focus()
	}

	for i := range m.headers {
		if m.headers[i].Name.Id == m.currentItemIndex {
			cmds[m.headers[i].Name.Id] = m.headers[i].Name.TextInput.Focus()
			m.headers[i].Name.TextInput.PromptStyle = STYLE_FOCUSED
			m.headers[i].Name.TextInput.TextStyle = STYLE_FOCUSED
			continue
		} else if m.headers[i].Value.Id == m.currentItemIndex {
			cmds[m.headers[i].Value.Id] = m.headers[i].Value.TextInput.Focus()
			m.headers[i].Value.TextInput.PromptStyle = STYLE_FOCUSED
			m.headers[i].Value.TextInput.TextStyle = STYLE_FOCUSED
			continue
		}

		m.headers[i].Name.TextInput.Blur()
		m.headers[i].Name.TextInput.PromptStyle = STYLE_NONE
		m.headers[i].Name.TextInput.TextStyle = STYLE_NONE

		m.headers[i].Value.TextInput.Blur()
		m.headers[i].Value.TextInput.PromptStyle = STYLE_NONE
		m.headers[i].Value.TextInput.TextStyle = STYLE_NONE
	}

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	s := strings.Builder{}

	{ // Draw Method menu
		if m.currentItemIndex != METHOD_FORM_ITEM_ID {
			s.WriteString(BLURRED_LABEL_METHOD)
		} else {
			s.WriteString(FOCUSED_LABEL_METHOD)
		}

		for i := 0; i < len(m.methods.Choices); i++ {
			if i != int(m.methods.SelectedIndex) {
				s.WriteString("( ) ")
			} else {
				s.WriteString("(•) ")
			}
			s.WriteString(m.methods.Choices[i])
			s.WriteString(" ")
		}
	}

	{ // Draw URL and body inputs
		if m.currentItemIndex != URL_FORM_ITEM_ID {
			s.WriteString(BLURRED_LABEL_URL)
		} else {
			s.WriteString(FOCUSED_LABEL_URL)
		}
		s.WriteString(m.urlInput.TextInput.View())

		if m.currentItemIndex != BODY_FORM_ITEM_ID {
			s.WriteString(BLURRED_LABEL_BODY)
		} else {
			s.WriteString(FOCUSED_LABEL_BODY)
		}
		s.WriteString(m.bodyInput.TextArea.View())
	}

	{ // Draw header inputs
		if (m.currentItemIndex > BODY_FORM_ITEM_ID) &&
			(m.currentItemIndex < SUBMIT_FORM_ITEM_ID) {
			s.WriteString(FOCUSED_LABEL_HEADERS)
		} else {
			s.WriteString(BLURRED_LABEL_HEADERS)
		}

		if len(m.headers) == 0 {
			s.WriteString("\nNone")
		}

		for i := 0; i < len(m.headers); i++ {
			s.WriteString("\nName: ")
			s.WriteString(m.headers[i].Name.TextInput.View())
			s.WriteString(" Value: ")
			s.WriteString(m.headers[i].Value.TextInput.View())
		}
	}

	if m.responseInfo == "" {
		{ // Draw footer
			s.WriteString("\n\n")

			if m.currentItemIndex != SUBMIT_FORM_ITEM_ID {
				s.WriteString(BLURRED_BTN_SUBMIT)
			} else {
				s.WriteString(FOCUSED_BTN_SUBMIT)
			}

			s.WriteString("\n\n{press esc to quit}\n\n")

			{ // Debug
				if DEBUG {
					s.WriteString("Current selection: ")
					s.WriteString(strconv.Itoa(int(m.currentItemIndex)))
				}
			}
		}
	} else {
		s.WriteString("\n\n")
		s.WriteString(STYLE_BLURRED.Render("RESPONSE:"))
		s.WriteString("\n\n")
		s.WriteString(m.responseInfo)
		s.WriteString(lipgloss.NewStyle().Width(m.currentWidth).Render(m.responseBody))
		s.WriteString("\n")
	}

	return s.String()
}

func main() {
	if os.Getenv("DEBUG") == "1" {
		DEBUG = true
	} else {
		DEBUG = false
	}

	if _, err := tea.NewProgram(initialModel()).Run(); err != nil {
		fmt.Printf("Could not start program: %s\n", err)
		os.Exit(1)
	}
}
