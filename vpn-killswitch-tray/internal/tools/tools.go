package tools

import "os/exec"

type Tools struct {
	Pkexec  bool `json:"pkexec"`
	Yad     bool `json:"yad"`
	Zenity  bool `json:"zenity"`
	KDialog bool `json:"kdialog"`
}

func Detect() Tools {
	return Tools{
		Pkexec:  exists("pkexec"),
		Yad:     exists("yad"),
		Zenity:  exists("zenity"),
		KDialog: exists("kdialog"),
	}
}

func exists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func (t Tools) DialogAvailable() bool {
	return t.Yad || t.Zenity || t.KDialog
}

func (t Tools) MutatingActionsAvailable() bool {
	return t.Pkexec
}
