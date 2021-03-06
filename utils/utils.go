package utils

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	cr "crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	mr "math/rand"
	"net"
	"os"
	"regexp"
	"strings"

	"github.com/hashicorp/yamux"
)

// AESKEY is used to encrypt shellcode on compiletime & decrypt it at runtime
var AESKEY = []byte("5339679294566578")
var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

// Exists ...
func Exists(name string) bool {
	_, err := os.Stat(name)
	return !os.IsNotExist(err)
}

// RandSeq ...
func RandSeq(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[mr.Intn(len(letters))]
	}
	return string(b)
}

// SplitAddress splits ipv4 or ipv6 address in port and ip part
func SplitAddress(addr string) (string, string) {
	ip := ""
	port := ""
	if strings.Contains(addr, "[") {
		// ipv6
		s := strings.Split(addr, "]")
		ip = s[0] + "]"
		port = strings.TrimLeft(s[1], ":")
	} else {
		// ipv4
		s := strings.Split(addr, ":")
		ip = s[0]
		port = s[1]
	}
	return ip, port
}

// Save base64 encoded file to disk
func Save(dst string, data string) bool {
	raw, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		log.Println(err)
		return false
	}
	err = ioutil.WriteFile(dst, raw, 0644)
	if err != nil {
		log.Println(err)
		return false
	}
	return true
}

// SaveRaw ...
func SaveRaw(dst string, data string) bool {
	err := ioutil.WriteFile(dst, []byte(data), 0644)
	if err != nil {
		log.Println(err)
		return false
	}
	return true
}

// Load file from disk and return base64 encoded representation
func Load(src string) (string, bool) {
	data, err := ioutil.ReadFile(src)
	if err != nil {
		log.Println(err)
		return "", false
	}
	b64 := base64.StdEncoding.EncodeToString(data)
	return b64, true
}

// LoadRaw ...
func LoadRaw(src string) ([]byte, bool) {
	data, err := ioutil.ReadFile(src)
	if err != nil {
		log.Println(err)
		return nil, false
	}
	return data, true
}

// CopyFile copies a file from a source path to a destination path
func CopyFile(src string, dst string) {
	// Read all content of src to data
	data, err := ioutil.ReadFile(src)
	if err != nil {
		log.Println(err)
	}
	// Write data to dst
	err = ioutil.WriteFile(dst, data, 0644)
	if err != nil {
		log.Println(err)
	}
}

// CopyIO copies data between a io.reader and a io.writer
func CopyIO(src, dest net.Conn) {
	defer src.Close()
	defer dest.Close()
	io.Copy(src, dest)
}

// UploadConnect ...
func UploadConnect(dst string, s *yamux.Session) {
	stream, err := s.Open()
	if err != nil {
		log.Println(err)
		return
	}
	defer stream.Close()
	line, err := ioutil.ReadAll(stream)
	if err != nil {
		log.Println(err)
		return
	}
	Save(dst, string(line))
}

// UploadConnectRaw ...
func UploadConnectRaw(s *yamux.Session) ([]byte, error) {
	stream, err := s.Open()
	if err != nil {
		return nil, err
	}
	defer stream.Close()
	line, err := ioutil.ReadAll(stream)
	if err != nil {
		return nil, err
	}
	raw, err := base64.StdEncoding.DecodeString(string(line))
	if err != nil {
		log.Println(err)
		return nil, err
	}
	return raw, nil
}

// DownloadConnect ...
func DownloadConnect(src string, s *yamux.Session) {
	stream, err := s.Open()
	if err != nil {
		log.Println(err)
		return
	}
	defer stream.Close()
	content, _ := Load(src)
	stream.Write([]byte(fmt.Sprintf("%s\r\n", content)))
}

// UploadListen ...
func UploadListen(src string, s *yamux.Session) {
	stream, err := s.Accept()
	if err != nil {
		log.Println(err)
		return
	}
	defer stream.Close()
	content, _ := Load(src)
	stream.Write([]byte(fmt.Sprintf("%s\r\n", content)))
}

// DownloadListen ...
func DownloadListen(dst string, s *yamux.Session) {
	stream, err := s.Accept()
	if err != nil {
		log.Println(err)
		return
	}
	defer stream.Close()
	line, err := ioutil.ReadAll(stream)
	if err != nil {
		log.Println(err)
		return
	}
	Save(dst, string(line))
}

// ByteToHex ...
func ByteToHex(s []byte) string {
	d := make([]byte, hex.DecodedLen(len(s)))
	n, err := hex.Decode(d, s)
	if err != nil {
		fmt.Println(err)
	}
	return fmt.Sprintf("%s", d[:n])
}

// Not sure where to put those, they are windows specific but their is no linux equivalent

// GetBuild ...
func GetBuild(raw string) string {
	// Microsoft Windows [Version 10.0.18363.778]
	var re = regexp.MustCompile(`(?P<build>[\d+\.]+)`)
	version := re.FindString(raw)
	return version
}

// GetHotfixes ...
func GetHotfixes(raw string) []string {
	// HOSTNAME Update KB4537572 NT AUTHORITY\SYSTEM 3/31/2020 12:00:00 AM
	kbs := []string{}
	var re = regexp.MustCompile(`(?m)(?P<kb>KB\d+)`)
	for _, match := range re.FindAllString(raw, -1) {
		kbs = append(kbs, match)
	}
	return kbs
}

// Encrypt ...
func Encrypt(key []byte, text []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	paddingLen := aes.BlockSize - (len(text) % aes.BlockSize)
	paddingText := bytes.Repeat([]byte{byte(paddingLen)}, paddingLen)
	textWithPadding := append(text, paddingText...)
	ciphertext := make([]byte, aes.BlockSize+len(textWithPadding))
	iv := ciphertext[:aes.BlockSize]
	if _, err := io.ReadFull(cr.Reader, iv); err != nil {
		return nil, err
	}
	cfbEncrypter := cipher.NewCFBEncrypter(block, iv)
	cfbEncrypter.XORKeyStream(ciphertext[aes.BlockSize:], textWithPadding)
	return ciphertext, nil
}

// Decrypt ...
func Decrypt(key []byte, text []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	if (len(text) % aes.BlockSize) != 0 {
		return nil, errors.New("wrong blocksize")
	}
	iv := text[:aes.BlockSize]
	decodedCipherMsg := text[aes.BlockSize:]
	cfbDecrypter := cipher.NewCFBDecrypter(block, iv)
	cfbDecrypter.XORKeyStream(decodedCipherMsg, decodedCipherMsg)
	length := len(decodedCipherMsg)
	paddingLen := int(decodedCipherMsg[length-1])
	result := decodedCipherMsg[:(length - paddingLen)]
	return result, nil
}
