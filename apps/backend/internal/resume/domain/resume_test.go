package domain

import (
	"errors"
	"testing"
)

func TestValidateUpload(t *testing.T) {
	name, err := ValidateUpload("../cv.pdf", "application/pdf", 1024)
	if err != nil || name != "cv.pdf" { t.Fatalf("name=%q err=%v", name, err) }
	for _, test := range []struct{name,mime string;size int64}{
		{"cv.exe","application/pdf",100}, {"cv.pdf","text/plain",100}, {"cv.pdf","application/pdf",0}, {"cv.pdf","application/pdf",MaxFileSize+1},
	} { if _, err := ValidateUpload(test.name,test.mime,test.size); !errors.Is(err,ErrInvalidResume) { t.Fatalf("accepted %+v",test) } }
}
