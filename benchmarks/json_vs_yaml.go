package main

import (
	"encoding/json"
	"testing"

	"gopkg.in/yaml.v2"
)

var testData = []byte(`
template: "article"
seo:
  title: "Your Page Title"
  description: "A brief description of your page content"
  keywords: "keyword1, keyword2, keyword3"
  author: "Your Name"
  robots: "index, follow"
head:
  - "<meta property=\"og:title\" content=\"Your Page Title\">"
  - "<meta property=\"og:description\" content=\"A brief description for social media sharing\">"
  - "<meta property=\"og:image\" content=\"https://yourdomain.com/image.jpg\">"
imports:
  css:
    - "https://cdn.example.com/some-stylesheet.css"
  js:
    - "https://cdn.example.com/some-script.js"
variables:
  showSidebar: true
  featuredImage: "/images/featured.jpg"
`)

var jsonData = []byte(`
{
  "template": "article",
  "seo": {
    "title": "Your Page Title",
    "description": "A brief description of your page content",
    "keywords": "keyword1, keyword2, keyword3",
    "author": "Your Name",
    "robots": "index, follow"
  },
  "head": [
    "<meta property=\"og:title\" content=\"Your Page Title\">",
    "<meta property=\"og:description\" content=\"A brief description for social media sharing\">",
    "<meta property=\"og:image\" content=\"https://yourdomain.com/image.jpg\">"
  ],
  "imports": {
    "css": [
      "https://cdn.example.com/some-stylesheet.css"
    ],
    "js": [
      "https://cdn.example.com/some-script.js"
    ]
  },
  "variables": {
    "showSidebar": true,
    "featuredImage": "/images/featured.jpg"
  }
}
`)

type ContentMeta struct {
	Template  string                 `json:"template" yaml:"template"`
	SEO       map[string]string      `json:"seo" yaml:"seo"`
	Head      []string               `json:"head" yaml:"head"`
	Imports   map[string][]string    `json:"imports" yaml:"imports"`
	Variables map[string]interface{} `json:"variables" yaml:"variables"`
}

func BenchmarkYAMLUnmarshal(b *testing.B) {
	var meta ContentMeta
	for i := 0; i < b.N; i++ {
		yaml.Unmarshal(testData, &meta)
	}
}

func BenchmarkJSONUnmarshal(b *testing.B) {
	var meta ContentMeta
	for i := 0; i < b.N; i++ {
		json.Unmarshal(jsonData, &meta)
	}
}