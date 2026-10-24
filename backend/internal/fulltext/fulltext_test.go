package fulltext

import "testing"

func TestSupportedURL(t *testing.T) {
	for _, raw := range []string{"http://openai.com/index/a/", "https://openai.com.evil.test/index/a/", "https://127.0.0.1/index/a/", "https://openai.com:8443/index/a/", "https://u:p@openai.com/index/a/", "https://openai.com/news/"} {
		if SupportedURL(raw) {
			t.Errorf("accepted %s", raw)
		}
	}
	if !SupportedURL("https://openai.com/index/article/") {
		t.Fatal("public article rejected")
	}
}
