package socks5

import (
	"io"
	"log"
	"net"
	"strconv"
	"sync"
)

// SOCKS5 константы
const (
	socks5Version = 0x05
	cmdConnect    = 0x01
	atypDomain    = 0x03
	atypIPv4      = 0x01
)

// StartSOCKS5Proxy запускает SOCKS5 прокси сервер
func StartSOCKS5Proxy(addr string) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	log.Printf("✅ SOCKS5 прокси запущен на %s", addr)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("SOCKS5 accept error: %v", err)
			continue
		}
		go handleSOCKS5(conn)
	}
}

func handleSOCKS5(client net.Conn) {
	defer client.Close()

	// Читаем握手
	buf := make([]byte, 256)
	_, err := client.Read(buf)
	if err != nil || buf[0] != socks5Version {
		return
	}

	// Ответ на握手
	_, err = client.Write([]byte{socks5Version, 0x00})
	if err != nil {
		return
	}

	// Читаем запрос
	_, err = client.Read(buf)
	if err != nil || buf[1] != cmdConnect {
		return
	}

	// Получаем адрес
	var host string
	var port int

	switch buf[3] {
	case atypIPv4:
		host = net.IP(buf[4:8]).String()
		port = int(buf[8])<<8 | int(buf[9])
	case atypDomain:
		domainLen := int(buf[4])
		host = string(buf[5 : 5+domainLen])
		port = int(buf[5+domainLen])<<8 | int(buf[6+domainLen])
	default:
		return
	}

	log.Printf("SOCKS5: %s -> %s:%d", client.RemoteAddr(), host, port)

	// Подключаемся к целевому хосту
	target, err := net.Dial("tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		client.Write([]byte{socks5Version, 0x04, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
		return
	}
	defer target.Close()

	// Отправляем ответ об успешном подключении
	_, err = client.Write([]byte{socks5Version, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
	if err != nil {
		return
	}

	// Передаём данные между клиентом и сервером
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		io.Copy(target, client)
	}()
	go func() {
		defer wg.Done()
		io.Copy(client, target)
	}()

	wg.Wait()
}