package swarm

import (
	"sort"
	"strings"
	"sync"

	gizaktermui "github.com/gizak/termui"
	"github.com/moncho/dry/appui"
	"github.com/moncho/dry/docker"
	"github.com/moncho/dry/ui"
	"github.com/moncho/dry/ui/termui"
)

//TasksWidget shows a service's task information
type TasksWidget struct {
	header               *termui.TableHeader
	filteredRows         []*TaskRow
	totalRows            []*TaskRow
	filterPattern        string
	height, width        int
	mounted              bool
	offset               int
	selectedIndex        int
	sortMode             docker.SortMode
	startIndex, endIndex int
	swarmClient          docker.SwarmAPI
	tableTitle           *termui.MarkupPar
	x, y                 int
	sync.RWMutex
}

//Filter applies the given filter to the container list
func (s *TasksWidget) Filter(filter string) {
	s.Lock()
	defer s.Unlock()
	s.filterPattern = filter
}

//OnEvent runs the given command
func (s *TasksWidget) OnEvent(event appui.EventCommand) error {
	if s.RowCount() > 0 {
		return event(s.filteredRows[s.selectedIndex].task.ID)
	}
	return nil
}

//RowCount returns the number of rowns of this widget.
func (s *TasksWidget) RowCount() int {
	return len(s.filteredRows)
}

//Sort rotates to the next sort mode.
//SortByTaskService -> SortByTaskImage -> SortByTaskDesiredState -> SortByTaskState -> SortByTaskService
func (s *TasksWidget) Sort() {
	s.Lock()
	defer s.Unlock()
	switch s.sortMode {
	case docker.SortByTaskService:
		s.sortMode = docker.SortByTaskImage
	case docker.SortByTaskImage:
		s.sortMode = docker.SortByTaskDesiredState
	case docker.SortByTaskDesiredState:
		s.sortMode = docker.SortByTaskState
	case docker.SortByTaskState:
		s.sortMode = docker.SortByTaskService
	}
}

//Unmount marks this widget as unmounted
func (s *TasksWidget) Unmount() error {
	s.Lock()
	defer s.Unlock()

	s.mounted = false
	return nil

}

//Align aligns rows
func (s *TasksWidget) align() {
	x := s.x
	width := s.width

	s.tableTitle.SetX(x)
	s.tableTitle.SetWidth(width)

	s.header.SetWidth(width)
	s.header.SetX(x)

	for _, task := range s.totalRows {
		task.SetX(x)
		task.SetWidth(width)
	}

}

func (s *TasksWidget) filterRows() {

	if s.filterPattern != "" {
		var rows []*TaskRow

		for _, row := range s.totalRows {
			if appui.RowFilters.ByPattern(s.filterPattern)(row) {
				rows = append(rows, row)
			}
		}
		s.filteredRows = rows
	} else {
		s.filteredRows = s.totalRows
	}
}

func (s *TasksWidget) calculateVisibleRows() {

	count := s.RowCount()

	//no screen
	if s.height < 0 || count == 0 {
		s.startIndex = 0
		s.endIndex = 0
		return
	}
	selected := s.selectedIndex
	//everything fits
	if count <= s.height {
		s.startIndex = 0
		s.endIndex = count
		return
	}
	//at the the start
	if selected == 0 {
		s.startIndex = 0
		s.endIndex = s.height
	} else if selected >= count-1 { //at the end
		s.startIndex = count - s.height
		s.endIndex = count
	} else if selected == s.endIndex { //scroll down by one
		s.startIndex++
		s.endIndex++
	} else if selected <= s.startIndex { //scroll up by one
		s.startIndex--
		s.endIndex--
	} else if selected > s.endIndex { // scroll
		s.startIndex = selected - s.height
		s.endIndex = selected
	}
}

//prepareForRendering sets the internal state of this widget so it is ready for
//rendering (i.e. Buffer()).
func (s *TasksWidget) prepareForRendering() {
	s.sortRows()
	s.filterRows()
	index := ui.ActiveScreen.Cursor.Position()
	if index < 0 {
		index = 0
	} else if index > s.RowCount() {
		index = s.RowCount() - 1
	}
	s.selectedIndex = index
	s.calculateVisibleRows()
}

func (s *TasksWidget) updateHeader() {
	sortMode := s.sortMode

	for _, c := range s.header.Columns {
		colTitle := c.Text
		var header appui.SortableColumnHeader
		if strings.Contains(colTitle, appui.DownArrow) {
			colTitle = colTitle[appui.DownArrowLength:]
		}
		for _, h := range taskTableHeaders {
			if colTitle == h.Title {
				header = h
				break
			}
		}
		if header.Mode == sortMode {
			c.Text = appui.DownArrow + colTitle
		} else {
			c.Text = colTitle
		}

	}

}

func (s *TasksWidget) visibleRows() []*TaskRow {
	return s.filteredRows[s.startIndex:s.endIndex]
}

func (s *TasksWidget) sortRows() {
	rows := s.totalRows
	mode := s.sortMode
	if mode == docker.NoSortTask {
		return
	}
	var sortAlg func(i, j int) bool
	switch mode {
	case docker.SortByTaskImage:
		sortAlg = func(i, j int) bool {
			return rows[i].Image.Text < rows[j].Image.Text
		}
	case docker.SortByTaskService:
		sortAlg = func(i, j int) bool {
			return rows[i].Name.Text < rows[j].Name.Text
		}
	case docker.SortByTaskState:
		sortAlg = func(i, j int) bool {
			return rows[i].CurrentState.Text < rows[j].CurrentState.Text
		}
	case docker.SortByTaskDesiredState:
		sortAlg = func(i, j int) bool {
			return rows[i].DesiredState.Text < rows[j].DesiredState.Text
		}

	}
	sort.SliceStable(rows, sortAlg)
}

var taskTableHeaders = []appui.SortableColumnHeader{
	{Title: "NAME", Mode: docker.SortByTaskService},
	{Title: "IMAGE", Mode: docker.SortByTaskImage},
	{Title: "NODE", Mode: docker.NoSortTask},
	{Title: "DESIRED STATE", Mode: docker.SortByTaskDesiredState},
	{Title: "CURRENT STATE", Mode: docker.SortByTaskState},
	{Title: "ERROR", Mode: docker.NoSortTask},
	{Title: "PORTS", Mode: docker.NoSortTask},
}

func taskTableHeader() *termui.TableHeader {

	header := termui.NewHeader(appui.DryTheme)
	header.ColumnSpacing = appui.DefaultColumnSpacing
	header.AddColumn(taskTableHeaders[0].Title)
	header.AddColumn(taskTableHeaders[1].Title)
	header.AddColumn(taskTableHeaders[2].Title)
	header.AddFixedWidthColumn(taskTableHeaders[3].Title, 13)
	header.AddFixedWidthColumn(taskTableHeaders[4].Title, 22)
	header.AddColumn(taskTableHeaders[5].Title)
	header.AddColumn(taskTableHeaders[6].Title)

	return header
}

func createStackTableTitle() *termui.MarkupPar {
	p := termui.NewParFromMarkupText(appui.DryTheme, "")
	p.Bg = gizaktermui.Attribute(appui.DryTheme.Bg)
	p.TextBgColor = gizaktermui.Attribute(appui.DryTheme.Bg)
	p.TextFgColor = gizaktermui.Attribute(appui.DryTheme.Info)
	p.Border = false

	return p
}
