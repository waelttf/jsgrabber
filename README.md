# jsgrabber
jsgrabber is a Go-based CLI tool that automates the end-to-end process of discovering, downloading, beautifying, and analyzing JavaScript files for potential endpoints and secrets essential for security researchers.
---

## 🔧 Features

- 🎯 Discover JS files using tools like `katana`, `gau`, `hakrawler`, `waybackurls`, and `getJS`
- ⚡ Concurrently download and deduplicate all JS files
- 🎨 Beautify JavaScript files using `js-beautify`
- 🔍 Extract endpoints with **LinkFinder**
- 🗝️ Identify secrets with **SecretFinder**
- 🗂 Structured folder output per target (clean recon data organization)

---
## 🔧 Installation
```bash
git clone https://github.com/yourusername/jsgrabber.git
cd jsgrabber
go build -o jsgrabber jsgrabber.go
```

## 📦 Requirements

Make sure the following tools are installed and in your `$PATH`:

- [katana](https://github.com/projectdiscovery/katana)
- [gau](https://github.com/lc/gau)
- [hakrawler](https://github.com/hakluke/hakrawler)
- [waybackurls](https://github.com/tomnomnom/waybackurls)
- [getJS](https://github.com/003random/getJS)
- [js-beautify](https://www.npmjs.com/package/js-beautify) `npm install -g js-beautify`
- [LinkFinder](https://github.com/GerbenJavado/LinkFinder)
- [SecretFinder](https://github.com/m4ll0k/SecretFinder)

---

## 🚀 Usage

```bash
go build jsgrabber.go
./jsgrabber -d example.com -i -l
```

## ⚠️ Disclaimer
This tool is intended for authorized testing and educational purposes only. Always get permission before targeting systems with this tool.
