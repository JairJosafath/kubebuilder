package main
import (
 "context"
 "bytes"
 "encoding/json"
 "errors"
 "fmt"
 "io"
 "net/http"
 "os"
 "regexp"
 "strings"
 "time"
 "github.com/jairjosafath/operator/internal/emoji"
)
type traceTransport struct { base http.RoundTripper; count int; key string }
func(t *traceTransport) RoundTrip(r *http.Request)(*http.Response,error) {
 t.count++
 resp,err:=t.base.RoundTrip(r)
 if err!=nil {fmt.Printf("request=%d transport_failed=true\n",t.count)} else {fmt.Printf("time=%s request=%d bytes=%d status=%d\n",time.Now().UTC().Format(time.RFC3339),t.count,r.ContentLength,resp.StatusCode)}
 if resp!=nil && resp.StatusCode!=200 {
  body,_:=io.ReadAll(io.LimitReader(resp.Body,128*1024+1));resp.Body.Close();resp.Body=io.NopCloser(bytes.NewReader(body))
  var result struct{Message string `json:"message"`; Error json.RawMessage `json:"error"`}; if json.Unmarshal(body,&result)!=nil{result.Message=string(body)}
  message:=strings.ReplaceAll(result.Message+" "+string(result.Error),t.key,"[REDACTED]");message=regexp.MustCompile(`[A-Za-z0-9_+/=-]{32,}`).ReplaceAllString(message,"[REDACTED]");if len(message)>700{message=message[:700]}
  fmt.Printf("content_type=%q error_detail=%q\n",resp.Header.Get("Content-Type"),message)
 }
 return resp,err
}
func main(){
 time.Sleep(30*time.Second)
 env,err:=os.ReadFile("/proc/128306/environ");if err!=nil{fmt.Println("Manager environment unavailable");return}
 var key,model string
 for _,v:=range strings.Split(string(env),"\x00"){if strings.HasPrefix(v,"JEV_API_KEY="){key=strings.TrimSpace(strings.TrimPrefix(v,"JEV_API_KEY="))};if strings.HasPrefix(v,"JEV_MODEL="){model=strings.TrimSpace(strings.TrimPrefix(v,"JEV_MODEL="))}}
 if key=="" {fmt.Println("No configured key");return}
 tracer:=&traceTransport{base:http.DefaultTransport,key:key};http.DefaultTransport=tracer
 client:=emoji.NewClient(key,model)
 ctx,cancel:=context.WithTimeout(context.Background(),10*time.Minute);defer cancel()
 for {
  matches,err:=client.Select(ctx,"Flying")
  if err==nil {fmt.Printf("selection_succeeded requests=%d matches=%v\n",tracer.count,matches);return}
  var limited *emoji.RateLimitError
  wait:=time.Minute
  if errors.As(err,&limited){wait=time.Until(limited.RetryAt)+time.Second}
  fmt.Println(err.Error())
  timer:=time.NewTimer(wait)
  select {case <-ctx.Done():timer.Stop();fmt.Println("Diagnostic deadline reached");return;case <-timer.C:}
 }
}
