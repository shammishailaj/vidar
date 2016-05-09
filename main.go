// This is free and unencumbered software released into the public
// domain.  For more information, see <http://unlicense.org> or the
// accompanying UNLICENSE file.

package main

import (
	"go/token"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/nelsam/gxui"
	"github.com/nelsam/gxui/drivers/gl"
	"github.com/nelsam/gxui/math"
	"github.com/nelsam/gxui/themes/basic"
	"github.com/nelsam/gxui/themes/dark"
	"github.com/nelsam/vidar/commander"
	"github.com/nelsam/vidar/commands"
	"github.com/nelsam/vidar/settings"
	"github.com/tmc/fonts"

	"github.com/nelsam/vidar/controller"
	"github.com/nelsam/vidar/editor"
	"github.com/nelsam/vidar/navigator"
	"github.com/spf13/cobra"
)

var (
	background = gxui.Gray10

	workingDir string
	cmd        *cobra.Command
	files      []string
)

func init() {
	cmd = &cobra.Command{
		Use:   "vidar [files...]",
		Short: "An experimental Go editor",
		Long:  "An editor for Go code, still in its infancy.  Basic editing of Go code is mostly complete, but there's still a potential for data loss.",
		Run: func(cmd *cobra.Command, args []string) {
			files = args
			gl.StartDriver(uiMain)
		},
	}
}

func main() {
	cmd.Execute()
}

func font(driver gxui.Driver) gxui.Font {
	desiredFonts := settings.DesiredFonts()
	if len(desiredFonts) == 0 {
		return nil
	}
	fontReader, err := fonts.Load(desiredFonts...)
	if err != nil {
		log.Printf("Error searching for fonts %v: %s", desiredFonts, err)
		return nil
	}
	if closer, ok := fontReader.(io.Closer); ok {
		defer closer.Close()
	}
	fontBytes, err := ioutil.ReadAll(fontReader)
	if err != nil {
		log.Printf("Failed to read font file: %s", err)
		return nil
	}
	font, err := driver.CreateFont(fontBytes, 12)
	if err != nil {
		log.Printf("Could not parse font: %s", err)
		return nil
	}
	return font
}

func uiMain(driver gxui.Driver) {
	theme := dark.CreateTheme(driver).(*basic.Theme)
	font := font(driver)
	if font == nil {
		font = theme.DefaultMonospaceFont()
	}
	theme.SetDefaultMonospaceFont(font)
	theme.SetDefaultFont(font)
	theme.WindowBackground = background

	// TODO: figure out a better way to get this resolution
	window := theme.CreateWindow(1600, 800, "Vidar - GXUI Go Editor")
	controller := controller.New(driver, theme)

	nav := navigator.New(driver, theme, controller)
	controller.SetNavigator(nav)

	editor := editor.New(driver, theme, theme.DefaultMonospaceFont())
	controller.SetEditor(editor)

	projTree := navigator.NewProjectTree(driver, theme)
	projects := navigator.NewProjectsPane(driver, theme, projTree.Frame())

	nav.Add(projects)
	nav.Add(projTree)

	nav.Resize(window.Size().H)
	window.OnResize(func() {
		nav.Resize(window.Size().H)
	})

	commander := commander.New(driver, theme, controller)

	// TODO: Check the system's DPI settings for this value
	window.SetScale(1)

	window.AddChild(commander)

	// TODO: Store these in a config file or something
	mapFileCommands(commander, projTree, driver, theme)
	mapEditCommands(commander, driver, theme)
	mapViewCommands(commander)

	window.OnKeyDown(func(event gxui.KeyboardEvent) {
		if (event.Modifier.Control() || event.Modifier.Super()) && event.Key == gxui.KeyQ {
			os.Exit(0)
		}
		if event.Modifier == 0 && event.Key == gxui.KeyF11 {
			window.SetFullscreen(!window.Fullscreen())
		}
		if window.Focus() == nil {
			commander.KeyDown(event)
		}
	})
	window.OnKeyUp(func(event gxui.KeyboardEvent) {
		if window.Focus() == nil {
			commander.KeyPress(event)
		}
	})

	workingDir, err := os.Getwd()
	if err != nil {
		log.Printf("Failed to read working directory: %s", err)
		workingDir = os.Getenv("HOME")
	}
	for _, file := range files {
		filepath := filepath.Join(workingDir, file)
		commander.Controller().Editor().Open(filepath, token.Position{})
	}

	window.OnClose(driver.Terminate)
	window.SetPadding(math.Spacing{L: 10, T: 10, R: 10, B: 10})
}

func mapFileCommands(commander *commander.Commander, projTree *navigator.ProjectTree, driver gxui.Driver, theme *basic.Theme) {
	addProject := commands.NewProjectAdder(driver, theme)
	ctrlShiftN := gxui.KeyboardEvent{
		Key:      gxui.KeyN,
		Modifier: gxui.ModControl | gxui.ModShift,
	}
	supShiftN := gxui.KeyboardEvent{
		Key:      gxui.KeyN,
		Modifier: gxui.ModSuper | gxui.ModShift,
	}
	commander.Map(addProject, "File", ctrlShiftN, supShiftN)

	openProj := commands.NewProjectOpener(theme, projTree.Frame())
	ctrlShiftO := gxui.KeyboardEvent{
		Key:      gxui.KeyO,
		Modifier: gxui.ModControl | gxui.ModShift,
	}
	cmdShiftO := gxui.KeyboardEvent{
		Key:      gxui.KeyO,
		Modifier: gxui.ModSuper | gxui.ModShift,
	}
	commander.Map(openProj, "File", ctrlShiftO, cmdShiftO)

	openFile := commands.NewFileOpener(driver, theme)
	ctrlO := gxui.KeyboardEvent{
		Key:      gxui.KeyO,
		Modifier: gxui.ModControl,
	}
	supO := gxui.KeyboardEvent{
		Key:      gxui.KeyO,
		Modifier: gxui.ModSuper,
	}
	commander.Map(openFile, "File", ctrlO, supO)

	goimports := commands.NewGoImports(theme)
	save := commands.NewSave(theme)
	ctrlS := gxui.KeyboardEvent{
		Key:      gxui.KeyS,
		Modifier: gxui.ModControl,
	}
	supS := gxui.KeyboardEvent{
		Key:      gxui.KeyS,
		Modifier: gxui.ModSuper,
	}
	saveAndGoimports := commands.NewMulti(theme, goimports, save)
	commander.Map(saveAndGoimports, "File", ctrlS, supS)

	closeTab := commands.NewCloseTab()
	ctrlW := gxui.KeyboardEvent{
		Key:      gxui.KeyW,
		Modifier: gxui.ModControl,
	}
	supW := gxui.KeyboardEvent{
		Key:      gxui.KeyW,
		Modifier: gxui.ModSuper,
	}
	commander.Map(closeTab, "File", ctrlW, supW)

}

func mapEditCommands(commander *commander.Commander, driver gxui.Driver, theme *basic.Theme) {
	undo := commands.NewUndo()
	ctrlZ := gxui.KeyboardEvent{
		Key:      gxui.KeyZ,
		Modifier: gxui.ModControl,
	}
	supZ := gxui.KeyboardEvent{
		Key:      gxui.KeyZ,
		Modifier: gxui.ModSuper,
	}
	commander.Map(undo, "Edit", ctrlZ, supZ)

	redo := commands.NewRedo(theme)
	ctrlR := gxui.KeyboardEvent{
		Key:      gxui.KeyR,
		Modifier: gxui.ModControl,
	}
	supR := gxui.KeyboardEvent{
		Key:      gxui.KeyR,
		Modifier: gxui.ModSuper,
	}
	commander.Map(redo, "Edit", ctrlR, supR)

	find := commands.NewFinder(driver, theme)
	ctrlF := gxui.KeyboardEvent{
		Key:      gxui.KeyF,
		Modifier: gxui.ModControl,
	}
	supF := gxui.KeyboardEvent{
		Key:      gxui.KeyF,
		Modifier: gxui.ModSuper,
	}
	commander.Map(find, "Edit", ctrlF, supF)

	copy := commands.NewCopy(driver)
	ctrlC := gxui.KeyboardEvent{
		Key:      gxui.KeyC,
		Modifier: gxui.ModControl,
	}
	supC := gxui.KeyboardEvent{
		Key:      gxui.KeyC,
		Modifier: gxui.ModSuper,
	}
	commander.Map(copy, "Edit", ctrlC, supC)

	cut := commands.NewCut(driver)
	ctrlX := gxui.KeyboardEvent{
		Key:      gxui.KeyX,
		Modifier: gxui.ModControl,
	}
	supX := gxui.KeyboardEvent{
		Key:      gxui.KeyX,
		Modifier: gxui.ModSuper,
	}
	commander.Map(cut, "Edit", ctrlX, supX)

	paste := commands.NewPaste(driver, theme)
	ctrlV := gxui.KeyboardEvent{
		Key:      gxui.KeyV,
		Modifier: gxui.ModControl,
	}
	supV := gxui.KeyboardEvent{
		Key:      gxui.KeyV,
		Modifier: gxui.ModSuper,
	}
	commander.Map(paste, "Edit", ctrlV, supV)

	gotoLine := commands.NewGotoLine(theme)
	ctrlG := gxui.KeyboardEvent{
		Key:      gxui.KeyG,
		Modifier: gxui.ModControl,
	}
	supG := gxui.KeyboardEvent{
		Key:      gxui.KeyG,
		Modifier: gxui.ModSuper,
	}
	commander.Map(gotoLine, "Edit", ctrlG, supG)

	godef := commands.NewGotoDef(theme)
	ctrlShiftG := gxui.KeyboardEvent{
		Key:      gxui.KeyG,
		Modifier: gxui.ModControl | gxui.ModShift,
	}
	supShiftG := gxui.KeyboardEvent{
		Key:      gxui.KeyG,
		Modifier: gxui.ModSuper | gxui.ModShift,
	}
	commander.Map(godef, "Edit", ctrlShiftG, supShiftG)

	licenseUpdate := commands.NewLicenseHeaderUpdate(theme)
	ctrlShiftL := gxui.KeyboardEvent{
		Key:      gxui.KeyL,
		Modifier: gxui.ModControl | gxui.ModShift,
	}
	supShiftL := gxui.KeyboardEvent{
		Key:      gxui.KeyL,
		Modifier: gxui.ModSuper | gxui.ModShift,
	}
	commander.Map(licenseUpdate, "Edit", ctrlShiftL, supShiftL)

	goimports := commands.NewGoImports(theme)
	ctrlShiftF := gxui.KeyboardEvent{
		Key:      gxui.KeyF,
		Modifier: gxui.ModControl | gxui.ModShift,
	}
	supShiftF := gxui.KeyboardEvent{
		Key:      gxui.KeyF,
		Modifier: gxui.ModSuper | gxui.ModShift,
	}
	commander.Map(goimports, "Edit", ctrlShiftF, supShiftF)

	togglecomments := commands.NewComments()
	ctrlComments := gxui.KeyboardEvent{
		Key:      gxui.KeySlash,
		Modifier: gxui.ModControl,
	}
	supComments := gxui.KeyboardEvent{
		Key:      gxui.KeySlash,
		Modifier: gxui.ModSuper,
	}
	commander.Map(togglecomments, "Edit", ctrlComments, supComments)
}

func mapViewCommands(commander *commander.Commander) {
	horizSplit := commands.NewHorizontalSplit()
	altH := gxui.KeyboardEvent{
		Key:      gxui.KeyH,
		Modifier: gxui.ModAlt,
	}
	commander.Map(horizSplit, "View", altH)

	vertSplit := commands.NewVerticalSplit()
	altV := gxui.KeyboardEvent{
		Key:      gxui.KeyV,
		Modifier: gxui.ModAlt,
	}
	commander.Map(vertSplit, "View", altV)
}
