//go:build windows

package engine

import (
	"os/exec"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

type processGuard struct {
	job windows.Handle
}

func configureProcess(cmd *exec.Cmd) (*processGuard, error) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_PROCESS_GROUP}
	return &processGuard{}, nil
}

func (g *processGuard) attach(cmd *exec.Cmd) error {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return err
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err = windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		windows.CloseHandle(job)
		return err
	}
	process, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(cmd.Process.Pid))
	if err != nil {
		windows.CloseHandle(job)
		return err
	}
	defer windows.CloseHandle(process)
	if err := windows.AssignProcessToJobObject(job, process); err != nil {
		windows.CloseHandle(job)
		return err
	}
	g.job = job
	return nil
}

func (g *processGuard) kill(cmd *exec.Cmd) error {
	if g.job != 0 {
		err := windows.CloseHandle(g.job)
		g.job = 0
		return err
	}
	if cmd != nil && cmd.Process != nil {
		return cmd.Process.Kill()
	}
	return nil
}
