// This is free and unencumbered software released into the public
// domain.  For more information, see <http://unlicense.org> or the
// accompanying UNLICENSE file.

package commander

import (
	"log"
	"sync"

	"github.com/nelsam/gxui"
	"github.com/nelsam/gxui/math"
	"github.com/nelsam/gxui/mixins/base"
	"github.com/nelsam/gxui/mixins/parts"
	"github.com/nelsam/gxui/themes/basic"
	"github.com/nelsam/vidar/commander/bind"
	"github.com/nelsam/vidar/controller"
	"github.com/nelsam/vidar/settings"
)

// Controller is a type which is used by the Commander to control the
// main UI.
type Controller interface {
	gxui.Control
	Editor() controller.Editor
}

// A commandMapping is a mapping between keyboard shortcuts (if any),
// a menu name, and a command.  The menu name is required.
type commandMapping struct {
	binding gxui.KeyboardEvent
	command bind.Command
}

// Commander is a gxui.LinearLayout that takes care of displaying the
// command utilities around a controller.
type Commander struct {
	base.Container
	parts.BackgroundBorderPainter

	theme *basic.Theme

	controller Controller
	box        *commandBox

	inputHandler InputHandler

	lock sync.RWMutex

	cmdStack [][]commandMapping
	commands []commandMapping
	menuBar  *menuBar
}

// New creates and initializes a *Commander, then returns it.
func New(driver gxui.Driver, theme *basic.Theme, controller Controller) *Commander {
	commander := &Commander{
		theme: theme,
	}
	commander.Container.Init(commander, theme)
	commander.BackgroundBorderPainter.Init(commander)
	commander.SetMouseEventTarget(true)
	commander.SetBackgroundBrush(gxui.TransparentBrush)
	commander.SetBorderPen(gxui.TransparentPen)

	mainLayout := theme.CreateLinearLayout()

	mainLayout.SetDirection(gxui.TopToBottom)
	mainLayout.SetSize(math.MaxSize)

	commander.controller = controller
	commander.menuBar = newMenuBar(commander, theme)
	commander.box = newCommandBox(driver, theme, commander.controller)

	mainLayout.AddChild(commander.menuBar)

	subLayout := theme.CreateLinearLayout()
	subLayout.SetDirection(gxui.BottomToTop)
	subLayout.AddChild(commander.box)
	subLayout.AddChild(commander.controller)
	mainLayout.AddChild(subLayout)
	commander.AddChild(mainLayout)
	return commander
}

func (c *Commander) DesiredSize(_, max math.Size) math.Size {
	return max
}

func (c *Commander) LayoutChildren() {
	maxSize := c.Size().Contract(c.Padding())
	for _, child := range c.Children() {
		margin := child.Control.Margin()
		desiredSize := child.Control.DesiredSize(math.ZeroSize, maxSize.Contract(margin))
		child.Control.SetSize(desiredSize)
	}
}

func (c *Commander) Paint(canvas gxui.Canvas) {
	rect := c.Size().Rect()
	c.BackgroundBorderPainter.PaintBackground(canvas, rect)
	c.PaintChildren.Paint(canvas)
	c.BackgroundBorderPainter.PaintBorder(canvas, rect)
}

func (c *Commander) archiveMap() []bind.Command {
	old := c.commands
	c.commands = nil
	c.cmdStack = append(c.cmdStack, old)

	used := make(map[bind.Command]struct{})
	var cmds []bind.Command
	for _, o := range old {
		if _, ok := used[o.command]; ok {
			continue
		}
		used[o.command] = struct{}{}
		cmds = append(cmds, o.command)

	}
	return cmds
}

// Push binds all bindables to c, pushing the previous
// binding down in the stack.
func (c *Commander) Push(bindables ...bind.Bindable) {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.menuBar.Clear()
	defer c.mapMenu()

	old := c.archiveMap()
	for _, cmd := range old {
		if cloneable, ok := cmd.(CloneableCommand); ok {
			cmd = cloneable.Clone()
		}
		c.bind(cmd, settings.Bindings(cmd.Name())...)
	}

	var hooks []bind.CommandHook
	for _, b := range bindables {
		switch src := b.(type) {
		case bind.CommandHook:
			hooks = append(hooks, src)
		case bind.Command:
			log.Printf("Binding command %s", src.Name())
			c.bind(src, settings.Bindings(src.Name())...)
		case InputHandler:
			c.inputHandler = src
		}
	}

	for _, h := range hooks {
		cmd := c.command(h.CommandName())
		if cmd == nil {
			log.Printf("Warning: could not find command %s to bind %s to", h.CommandName(), h.Name())
			continue
		}
		hooked, ok := cmd.(HookedCommand)
		if !ok {
			log.Printf("Warning: %s cannot bind to command %s (not a HookedCommand)", h.Name(), cmd.Name())
			continue
		}
		if err := hooked.Bind(h); err != nil {
			log.Printf("Warning: failed to bind hook %s to HookedCommand %s", h.Name(), hooked.Name())
		}
	}
}

func (c *Commander) mapMenu() {
	keys := make(map[bind.Command][]gxui.KeyboardEvent)
	var cmds []bind.Command
	for _, bound := range c.commands {
		if _, ok := keys[bound.command]; !ok {
			cmds = append(cmds, bound.command)
		}
		keys[bound.command] = append(keys[bound.command], bound.binding)
	}
	for _, cmd := range cmds {
		c.menuBar.Add(cmd, keys[cmd]...)
	}
}

// Pop pops the most recent call to Bind, restoring the
// previous bind.
func (c *Commander) Pop() {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.menuBar.Clear()
	defer c.mapMenu()

	newLen := len(c.cmdStack) - 1
	c.commands = c.cmdStack[newLen]
	c.cmdStack = c.cmdStack[:newLen]
}

func (c *Commander) bind(command bind.Command, binding ...gxui.KeyboardEvent) {
	for _, binding := range binding {
		if i := c.bindIdx(binding); i >= 0 {
			log.Printf("Warning: command %s is overriding command %s at binding %v", command.Name(), c.commands[i].command.Name(), binding)
			c.commands = append(c.commands[:i], c.commands[i+1:]...)
		}
		c.mapBinding(command, binding)
	}
}

func (c *Commander) bindIdx(binding gxui.KeyboardEvent) int {
	if binding.Key == gxui.KeyUnknown {
		return -1
	}
	for i, mapping := range c.commands {
		if mapping.binding == binding {
			return i
		}
	}
	return -1
}

func (c *Commander) mapBinding(command bind.Command, binding gxui.KeyboardEvent) {
	c.commands = append(c.commands, commandMapping{
		binding: binding,
		command: command,
	})
}

// Binding finds and returns the Command associated with bind.
func (c *Commander) Binding(binding gxui.KeyboardEvent) bind.Command {
	c.lock.RLock()
	defer c.lock.RUnlock()

	i := c.bindIdx(binding)
	if i < 0 {
		return nil
	}
	return c.commands[i].command
}

// Command looks up a bind.Command by name.
func (c *Commander) Command(name string) bind.Command {
	c.lock.RLock()
	defer c.lock.RUnlock()

	return c.command(name)
}

func (c *Commander) command(name string) bind.Command {
	for _, m := range c.commands {
		if m.command.Name() == name {
			return m.command
		}
	}
	return nil
}

// KeyPress handles key bindings for c.
func (c *Commander) KeyPress(event gxui.KeyboardEvent) (consume bool) {
	if event.Modifier == 0 && event.Key == gxui.KeyEscape {
		c.box.Clear()
		c.controller.Editor().Focus()
		return true
	}
	cmdDone := c.box.HasFocus() && event.Modifier == 0 && event.Key == gxui.KeyEnter
	if command := c.Binding(event); command != nil {
		c.box.Clear()
		if c.box.Run(command) {
			return true
		}
		cmdDone = true
	}
	if !cmdDone {
		return false
	}
	if executor, ok := c.box.Current().(bind.Executor); ok {
		c.Execute(executor)
	}
	c.box.Finish()
	return true
}

func (c *Commander) KeyStroke(event gxui.KeyStrokeEvent) (consume bool) {
	if event.Modifier&^gxui.ModShift != 0 {
		return false
	}
	e := c.controller.Editor().CurrentEditor()
	if e == nil || !e.HasFocus() {
		return false
	}
	c.inputHandler.HandleInput(e, event)
	return true
}

func (c *Commander) Execute(e bind.Executor) {
	if before, ok := e.(BeforeExecutor); ok {
		before.BeforeExec(c)
	}
	executed, _ := execute(e, c)
	if !executed {
		log.Printf("Warning: Executor of type %T ran without executing", e)
	}
}

func execute(executor bind.Executor, elem interface{}) (executed, consume bool) {
	executed, consume = executor.Exec(elem)
	if consume {
		return executed, consume
	}
	var childExecuted bool
	switch src := elem.(type) {
	case Elementer:
		for _, element := range src.Elements() {
			childExecuted, consume = execute(executor, element)
			executed = executed || childExecuted
			if consume {
				break
			}
		}
		return executed, consume
	case gxui.Parent:
		for _, child := range src.Children() {
			childExecuted, consume = execute(executor, child.Control)
			executed = executed || childExecuted
			if consume {
				break
			}
		}
		return executed, consume
	}
	return false, false
}
