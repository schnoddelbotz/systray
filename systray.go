/*
Package systray is a cross platfrom Go library to place an icon and menu in the
notification area.
Supports Windows, Mac OSX and Linux currently.
Methods can be called from any goroutine except Run(), which should be called
at the very beginning of main() to lock at main thread.
*/
package systray

import (
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/getlantern/golog"
)

var (
	hasStarted = int64(0)
	hasQuit    = int64(0)
)

const (
	ItemDefault        = 0
	ItemSeparator byte = 1 << iota
	ItemChecked
	ItemCheckable
	ItemDisabled
)

// MenuItem is used to keep track each menu item of systray
// Don't create it directly, use the one systray.AddMenuItem() returned
type MenuItem struct {
	// ClickedCh is the channel which will be notified when the menu item is clicked
	ClickedCh chan struct{}

	// id uniquely identify a menu item, not supposed to be modified
	id int32
	// menuId uniquely identify a menu the menu item belogns to
	menuId int32
	// title is the text shown on menu item
	title string
	// tooltip is the text shown when pointing to menu item
	tooltip string
	// disabled menu item is grayed out and has no effect when clicked
	disabled bool
	// checked menu item has a tick before the title
	checked       bool
	checkable     bool
	isSeparator   bool
	isSubmenu     bool
	isSubmenuItem bool
}

var (
	log = golog.LoggerFor("systray")

	systrayReady  func()
	systrayExit   func()
	menuItems     = make(map[int32]*MenuItem)
	menuItemsLock sync.RWMutex

	currentID = int32(-1)
)

// Run initializes GUI and starts the event loop, then invokes the onReady
// callback.
// It blocks until systray.Quit() is called.
// Should be called at the very beginning of main() to lock at main thread.
func Run(onReady func(), onExit func()) {
	runtime.LockOSThread()
	atomic.StoreInt64(&hasStarted, 1)

	if onReady == nil {
		systrayReady = func() {}
	} else {
		// Run onReady on separate goroutine to avoid blocking event loop
		readyCh := make(chan interface{})
		go func() {
			<-readyCh
			onReady()
		}()
		systrayReady = func() {
			close(readyCh)
		}
	}

	// unlike onReady, onExit runs in the event loop to make sure it has time to
	// finish before the process terminates
	if onExit == nil {
		onExit = func() {}
	}
	systrayExit = onExit

	nativeLoop()
}

// Quit the systray
func Quit() {
	if atomic.LoadInt64(&hasStarted) == 1 && atomic.CompareAndSwapInt64(&hasQuit, 0, 1) {
		quit()
	}
}

// AddMenuItem adds menu item with designated title and tooltip, returning a channel
// that notifies whenever that menu item is clicked.
//
// It can be safely invoked from different goroutines.
func AddMenuItem(title string, tooltip string, flagsArr ...byte) *MenuItem {
	id := atomic.AddInt32(&currentID, 1)

	flags := byte(0)
	if len(flagsArr) > 0 {
		flags = flagsArr[0]
		if ItemSeparator&flags != 0 {
			// remove other flags if separator
			flags = ItemSeparator
		} else if ItemCheckable&flags == 0 {
			// remove "checked" flag if not checkable
			flags &= ^ItemChecked
		}
	}

	item := &MenuItem{
		ClickedCh:     make(chan struct{}),
		id:            id,
		title:         title,
		tooltip:       tooltip,
		isSubmenuItem: false,
		isSubmenu:     false,
		isSeparator:   ItemSeparator&flags != 0,
		checkable:     ItemCheckable&flags != 0,
		checked:       ItemChecked&flags != 0,
		disabled:      ItemDisabled&flags != 0,
	}

	item.update()
	return item
}

// AddSubMenu adds a sub menu to the systray and
// and returns an id to access to accesss the menu
func AddSubMenu(title string) *MenuItem {
	subMenuId := atomic.AddInt32(&currentID, 1)
	createSubMenu(subMenuId)

	item := &MenuItem{
		ClickedCh:     make(chan struct{}),
		id:            atomic.AddInt32(&currentID, 1),
		title:         title,
		menuId:        subMenuId,
		isSubmenuItem: false,
	}
	if atomic.LoadInt64(&hasStarted) == 1 {
		addSubmenuToTray(item)
	}
	item.update()
	return item
}

func (sitem *MenuItem) AddSubMenuItem(title, tooltip string, flags byte) *MenuItem {
	if ItemSeparator&flags != 0 {
		// remove other flags if separator
		flags = ItemSeparator
	} else if ItemCheckable&flags == 0 {
		// remove "checked" flag if not checkable
		flags &= ^ItemChecked
	}

	item := &MenuItem{
		ClickedCh:     make(chan struct{}),
		id:            atomic.AddInt32(&currentID, 1),
		title:         title,
		tooltip:       tooltip,
		menuId:        sitem.menuId,
		isSeparator:   ItemSeparator&flags != 0,
		isSubmenuItem: true,
		checkable:     ItemCheckable&flags != 0,
		checked:       ItemChecked&flags != 0,
		disabled:      ItemDisabled&flags != 0,
	}

	item.update()
	return item
}

// AddSeparator adds a separator bar to the menu
func AddSeparator() {
	addSeparator(atomic.AddInt32(&currentID, 1))
}

// SetTitle set the text to display on a menu item
func (item *MenuItem) SetTitle(title string) {
	item.title = title
	item.update()
}

// SetTooltip set the tooltip to show when mouse hover
func (item *MenuItem) SetTooltip(tooltip string) {
	item.tooltip = tooltip
	item.update()
}

// Disabled checkes if the menu item is disabled
func (item *MenuItem) Disabled() bool {
	return item.disabled
}

// Enable a menu item regardless if it's previously enabled or not
func (item *MenuItem) Enable() {
	item.disabled = false
	item.update()
}

// Disable a menu item regardless if it's previously disabled or not
func (item *MenuItem) Disable() {
	item.disabled = true
	item.update()
}

// Hide hides a menu item
func (item *MenuItem) Hide() {
	hideMenuItem(item)
}

// Show shows a previously hidden menu item
func (item *MenuItem) Show() {
	showMenuItem(item)
}

// Checked returns if the menu item has a check mark
func (item *MenuItem) Checked() bool {
	return item.checked
}

// Check a menu item regardless if it's previously checked or not
func (item *MenuItem) Check() {
	item.checked = true
	item.update()
}

// Uncheck a menu item regardless if it's previously unchecked or not
func (item *MenuItem) Uncheck() {
	item.checked = false
	item.update()
}

// update propogates changes on a menu item to systray
func (item *MenuItem) update() {
	menuItemsLock.Lock()
	defer menuItemsLock.Unlock()
	menuItems[item.id] = item
	addOrUpdateMenuItem(item)
}

func systrayMenuItemSelected(id int32) {
	menuItemsLock.RLock()
	item := menuItems[id]
	menuItemsLock.RUnlock()
	select {
	case item.ClickedCh <- struct{}{}:
	// in case no one waiting for the channel
	default:
	}
}
