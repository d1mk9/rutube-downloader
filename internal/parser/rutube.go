package parser

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/grafov/m3u8"
)

var httpClient = &http.Client{Timeout: 30 * time.Second}

type playOptions struct {
	Title         string `json:"title"`
	VideoBalancer struct {
		M3u8 string `json:"m3u8"`
	} `json:"video_balancer"`
}

// ExtractMP4 качает RuTube-ролик, сохраняет в downloads/ и
// возвращает только имя итогового файла (без папки).
func ExtractMP4(videoURL string) (string, error) {
	id, err := extractID(videoURL)
	if err != nil {
		return "", err
	}
	opts, err := fetchOptions(id)
	if err != nil {
		return "", err
	}
	variantURL, err := pickBestVariant(opts.VideoBalancer.M3u8)
	if err != nil {
		return "", err
	}

	segs, base, err := fetchSegments(variantURL)
	if err != nil {
		return "", err
	}

	tmp, _ := os.MkdirTemp("", "rutube-*")
	defer os.RemoveAll(tmp)

	if err := downloadAll(segs, base, tmp); err != nil {
		return "", err
	}

	joined := filepath.Join(tmp, "joined.ts")
	if err := concatTS(segs, tmp, joined); err != nil {
		return "", err
	}

	fileName := sanitize(opts.Title) + ".mp4"

	// итоговый путь — downloads/…
	outPath := filepath.Join("downloads", fileName)
	if err := os.MkdirAll("downloads", 0o755); err != nil {
		return "", err
	}

	if err := ffmpegCopy(joined, outPath); err != nil {
		return "", err
	}

	// удаляем через 5 минут
	go func(p string) {
		time.Sleep(5 * time.Minute)
		_ = os.Remove(p)
	}(outPath)

	return fileName, nil
}

// --- helpers --------------------------------------------------------------

func extractID(input string) (string, error) {
	input = strings.TrimSpace(input)
	re := regexp.MustCompile(`(?i)^https?://rutube\.ru/video/([a-f0-9]{32})/?$`)
	m := re.FindStringSubmatch(input)
	if len(m) < 2 {
		return "", errors.New("не смог распознать ID видео")
	}
	return m[1], nil
}

func fetchOptions(id string) (*playOptions, error) {
	// Сначала пробуем новый init-эндпоинт
	if po, err := fetchOptionsInit(id); err == nil && po.VideoBalancer.M3u8 != "" {
		log.Println("✅ Использован init-эндпоинт")
		return po, nil
	} else {
		log.Printf("⚠️ init-эндпоинт не сработал: %v", err)
	}

	// Фолбэк на старый play/options
	url := fmt.Sprintf("https://rutube.ru/api/play/options/%s/?no_404=true&referer=https%%3A%%2F%%2Frutube.ru", id)
	r, err := httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()
	if r.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("play/options http %d", r.StatusCode)
	}
	var po playOptions
	if err := json.NewDecoder(r.Body).Decode(&po); err != nil {
		return nil, err
	}
	if po.VideoBalancer.M3u8 == "" {
		return nil, errors.New("video_balancer.m3u8 пустой (fallback)")
	}
	return &po, nil
}

func fetchOptionsInit(id string) (*playOptions, error) {
	url := fmt.Sprintf("https://rutube.ru/api/video/%s/init", id)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0") // Без него бывает 403

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("init http %d", resp.StatusCode)
	}

	var full map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&full); err != nil {
		return nil, err
	}

	// Извлекаем m3u8 вручную
	m3u8url, ok := full["video_balancer"].(map[string]interface{})["m3u8"].(string)
	if !ok || m3u8url == "" {
		return nil, errors.New("m3u8 не найден в init")
	}

	title, _ := full["title"].(string)

	return &playOptions{
		Title: title,
		VideoBalancer: struct {
			M3u8 string `json:"m3u8"`
		}{
			M3u8: m3u8url,
		},
	}, nil
}

func pickBestVariant(m3u8url string) (string, error) {
	m3u8url = cleanPath(m3u8url)
	resp, err := httpClient.Get(m3u8url)
	if err != nil {
		return "", fmt.Errorf("ошибка загрузки m3u8: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("m3u8 http %d", resp.StatusCode)
	}

	// Пытаемся как master
	pl := m3u8.NewMasterPlaylist()
	if err := pl.DecodeFrom(resp.Body, true); err == nil && len(pl.Variants) > 0 {
		sort.Slice(pl.Variants, func(i, j int) bool {
			return pl.Variants[i].Bandwidth > pl.Variants[j].Bandwidth
		})
		return resolveURL(m3u8url, pl.Variants[0].URI), nil
	}

	// Похоже, это сразу media-плейлист — возвращаем исходную ссылку
	log.Println("⚠️ M3U8 не содержит вариантов, используем напрямую как media")
	return m3u8url, nil
}

func fetchSegments(variant string) ([]string, string, error) {
	variant = cleanPath(variant)
	resp, err := httpClient.Get(variant)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("variant http %d", resp.StatusCode)
	}
	media, typ, err := m3u8.DecodeFrom(resp.Body, true)
	if err != nil {
		return nil, "", err
	}
	if typ != m3u8.MEDIA {
		return nil, "", errors.New("ожидался media playlist")
	}
	mpl := media.(*m3u8.MediaPlaylist)

	var segs []string
	for _, s := range mpl.Segments {
		if s != nil {
			segs = append(segs, cleanPath(s.URI))
		}
	}

	baseURL, _ := url.Parse(variant)
	baseURL.Path = filepath.Dir(baseURL.Path) + "/"
	return segs, baseURL.String(), nil
}

func downloadAll(list []string, base, dir string) error {
	w := runtime.GOMAXPROCS(0) * 2
	sem := make(chan struct{}, w)
	var wg sync.WaitGroup
	var first error
	var mu sync.Mutex

	for i, n := range list {
		if n == "" {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int, name string) {
			defer func() { <-sem; wg.Done() }()
			if err := downloadOne(resolveURL(base, name), filepath.Join(dir, fmt.Sprintf("seg%05d.ts", idx))); err != nil {
				mu.Lock()
				if first == nil {
					first = err
				}
				mu.Unlock()
			}
		}(i, n)
	}
	wg.Wait()
	return first
}

func downloadOne(u, dst string) error {
	r, err := httpClient.Get(u)
	if err != nil {
		return err
	}
	defer r.Body.Close()
	if r.StatusCode != http.StatusOK {
		return fmt.Errorf("seg http %d", r.StatusCode)
	}
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, r.Body)
	return err
}

func concatTS(list []string, dir, out string) error {
	merged, err := os.Create(out)
	if err != nil {
		return err
	}
	defer merged.Close()
	for i := range list {
		seg := filepath.Join(dir, fmt.Sprintf("seg%05d.ts", i))
		in, err := os.Open(seg)
		if err != nil {
			return err
		}
		if _, err := io.Copy(merged, in); err != nil {
			in.Close()
			return err
		}
		in.Close()
	}
	return nil
}

func ffmpegCopy(ts, mp4 string) error {
	ffmpegPath := "ffmpeg"
	if runtime.GOOS == "windows" {
		ffmpegPath = "ffmpeg/bin/ffmpeg.exe"
	}
	cmd := exec.Command(ffmpegPath, "-y", "-i", ts, "-c", "copy", mp4)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func resolveURL(master, ref string) string {
	master = cleanPath(master)
	ref = cleanPath(ref)

	// Если ссылка уже абсолютная — возвращаем как есть
	if strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") {
		return ref
	}

	mu, _ := url.Parse(master)
	ru, _ := url.Parse(ref)
	return mu.ResolveReference(ru).String()
}

func cleanPath(p string) string {
	d, _ := url.PathUnescape(p)
	return strings.ReplaceAll(d, "\\", "/")
}

func sanitize(s string) string {
	s = strings.TrimSpace(s)

	// Заменим опасные символы на подчёркивание
	re := regexp.MustCompile(`[<>:"/\\|?*]+`)
	s = re.ReplaceAllString(s, "_")

	// Удалим управляющие символы и переводы строк
	s = strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, s)

	// Ограничим длину имени (Telegram не любит очень длинные)
	const maxLength = 80
	runes := []rune(s)
	if len(runes) > maxLength {
		s = string(runes[:maxLength])
	}

	// Если всё удалилось — подставим временное имя
	if s == "" {
		s = fmt.Sprintf("rutube_%d", time.Now().Unix())
	}
	return s
}
