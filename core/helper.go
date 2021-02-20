package core

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"

	"github.com/dsnet/compress/brotli"
	ll "github.com/evilsocket/islazy/log"
)

type Response struct {
	*http.Response
}

func (response *Response) Unpack() (buffer []byte, err error) {

	var rc io.ReadCloser

	switch response.Header.Get("Content-Encoding") {
	case "x-gzip":
		fallthrough
	case "gzip":
		rc, err = gzip.NewReader(response.Body)
		if err != io.EOF {
			buffer, _ = ioutil.ReadAll(rc)
			defer rc.Close()
		} else {
			err = nil
		}
	case "br":
		c := brotli.ReaderConfig{}
		rc, err = brotli.NewReader(response.Body, &c)
		buffer, _ = ioutil.ReadAll(rc)
		defer rc.Close()
	case "deflate":
		rc = flate.NewReader(response.Body)
		buffer, _ = ioutil.ReadAll(rc)
		defer rc.Close()
	case "compress":
		fallthrough
	default:
		rc = response.Body
		buffer, _ = ioutil.ReadAll(rc)
		defer rc.Close()
	}
	return
}

func (response *Response) Pack(buffer []byte) (err error) {

	switch response.Header.Get("Content-Encoding") {
	case "x-gzip":
		fallthrough
	case "gzip":
		buffer, err = packGzip(buffer)
	case "deflate":
		buffer, err = packDeflate(buffer)
	case "br":
		response.Header.Set("Content-Encoding", "deflate")
		buffer, err = packDeflate(buffer)
	default:
		// just don't pack
	}

	if err != nil {
		ll.Error("[Pack] Error packing with %s: %s", response.Header.Get("Content-Encoding"), err)
	}

	body := ioutil.NopCloser(bytes.NewReader(buffer))
	response.Body = body
	response.ContentLength = int64(len(buffer))
	response.Header.Set("Content-Length", strconv.Itoa(len(buffer)))
	return nil
}

func packGzip(i []byte) ([]byte, error) {

	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	if _, err := gz.Write(i); err != nil {
		return nil, err
	}
	if err := gz.Flush(); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func packDeflate(i []byte) ([]byte, error) {

	var b bytes.Buffer
	zz, err := flate.NewWriter(&b, 0)

	if err != nil {
		return nil, err
	}
	if _, err = zz.Write(i); err != nil {
		return nil, err
	}
	if err := zz.Flush(); err != nil {
		return nil, err
	}
	if err := zz.Close(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

type iError interface {
	IsError() bool
}

// https://mikeschinkel.me/2019/gos-unfortunate-err-nil-idiom/
func IsError(err error) bool {
	if err == nil {
		return false
	}

	ei, ok := err.(iError)
	if !ok {
		return true
	}

	return ei.IsError()
}
