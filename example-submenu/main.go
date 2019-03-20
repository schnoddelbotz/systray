package main

import (
	"fmt"
	"github.com/getlantern/systray"
)

var (
	menuTitles          = []string{"minishift", "kuberenets", "kubedash", "kvirt"}
	submenus            = make(map[string]*systray.MenuItem)
	submenusToMenuItems = make(map[string]MenuAction)
)

func main() {
	systray.Run(onReady, onExit)
}

// MenuAction doc
type MenuAction struct {
	start *systray.MenuItem
	stop  *systray.MenuItem
}

func onReady() {
	systray.SetTitle("Awesome App")
	exit := systray.AddMenuItem("Exit", "", 0)
	systray.AddSeparator()
	systray.AddMenuItem("Enabled", "Enabled", 0)

	levelOne := systray.AddSubMenu("Disabled")
	levelOne.Disable()

	for _, menuTitle := range menuTitles {
		submenu := systray.AddSubMenu(menuTitle)
		startMenu := submenu.AddSubMenuItem("Start", "", 0)
		stopMenu := submenu.AddSubMenuItem("Stop", "", 0)
		submenus[menuTitle] = submenu
		submenusToMenuItems[menuTitle] = MenuAction{start: startMenu, stop: stopMenu}
	}

	go func() {
		<-exit.ClickedCh
		systray.Quit()
	}()

	for k, v := range submenusToMenuItems {
		fmt.Println(k)
		go func(submenu string, v MenuAction) {
			for {
				<-v.start.ClickedCh
				fmt.Println("start", submenu)
			}
		}(k, v)

		go func( submenu string, v MenuAction) {
			for {
				<-v.stop.ClickedCh
				fmt.Println("stop", submenu)
			}
		}(k, v)
	}

}

func onExit() {

}
