package httpapi
import "testing"
func TestValidMemoryPayload(t *testing.T){
 cases:=[]struct{name string;in input;want bool}{
 {"valid",input{Title:"hello",Content:"world"},true},
 {"blank title",input{Title:"   "},false},
 {"title too long",input{Title:string(make([]byte,201))},false},
 {"content too long",input{Title:"x",Content:string(make([]byte,100001))},false},
 }
 for _,tc:=range cases{t.Run(tc.name,func(t *testing.T){if got:=valid(tc.in);got!=tc.want{t.Fatalf("valid()=%v want %v",got,tc.want)}})}
}
