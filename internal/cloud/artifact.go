package cloud

import (
	"archive/zip"
	"bytes"
	"debug/elf"
	"io"
)

func preparedArchitecture(path string) string {
	executable, err := elf.Open(path)
	if err != nil {
		return ""
	}
	defer executable.Close()
	for _, p := range executable.Progs {
		if p.Type == elf.PT_INTERP {
			return ""
		}
	}
	switch executable.Machine {
	case elf.EM_AARCH64:
		return "arm64"
	case elf.EM_X86_64:
		return "x86_64"
	}
	return ""
}

func lambdaArchive(input io.ReadSeeker) ([]byte, error) {
	if _, err := input.Seek(0, 0); err != nil {
		return nil, err
	}
	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)
	header := &zip.FileHeader{Name: "bootstrap", Method: zip.Deflate}
	header.SetMode(0755)
	entry, err := archive.CreateHeader(header)
	if err != nil {
		return nil, err
	}
	if _, err = io.Copy(entry, input); err != nil {
		return nil, err
	}
	if err = archive.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}
