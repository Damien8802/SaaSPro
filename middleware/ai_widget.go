package middleware

import (
    "bytes"
    "strings"

    "github.com/gin-gonic/gin"
)

type responseWriter struct {
    gin.ResponseWriter
    body *bytes.Buffer
}

func (rw *responseWriter) Write(b []byte) (int, error) {
    return rw.body.Write(b)
}

func AIWidgetMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        // Только для HTML запросов
        if !strings.Contains(c.Request.Header.Get("Accept"), "text/html") {
            c.Next()
            return
        }
        
        rw := &responseWriter{ResponseWriter: c.Writer, body: bytes.NewBufferString("")}
        c.Writer = rw
        
        c.Next()
        
        if strings.Contains(c.Writer.Header().Get("Content-Type"), "text/html") && rw.body.Len() > 0 {
            html := rw.body.String()
            // Добавляем скрипт AI Assistant перед закрывающим </body>
            widgetScript := `<script src="/static/ai-assistant-full.js"></script>`
            html = strings.Replace(html, "</body>", widgetScript+"</body>", 1)
            c.Writer.Write([]byte(html))
        } else {
            c.Writer.Write(rw.body.Bytes())
        }
    }
}