//go:build windows

package i18n

import (
	"syscall"
	"unsafe"
)

func platformLocale() string {
	const localeNameMaxLength = 85
	buffer := make([]uint16, localeNameMaxLength)
	proc := syscall.NewLazyDLL("kernel32.dll").NewProc("GetUserDefaultLocaleName")
	result, _, _ := proc.Call(uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)))
	if result == 0 {
		return "en"
	}
	return syscall.UTF16ToString(buffer)
}
