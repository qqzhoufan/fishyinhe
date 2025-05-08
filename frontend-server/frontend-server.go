package main

import (
	"bufio" // 用于等待用户输入 (在错误时)
	"flag"  // 用于处理命令行参数
	"log"
	"net/http" // 用于 HTTP 服务
	"os"
	"path/filepath" // 用于处理文件路径
	"strings"       // 用于 spaMiddleware 中的路径检查
)

func main() {
	// 定义命令行参数
	// -p 指定端口号，默认为 "5680"
	// -d 指定要服务的目录名 (相对于可执行文件)，默认为 "dist"
	port := flag.String("p", "5680", "Port to serve on")
	directory := flag.String("d", "dist", "Directory to serve files from (relative to executable)")
	flag.Parse() // 解析命令行参数

	// 1. 获取可执行文件所在的目录
	exePath, err := os.Executable()
	if err != nil {
		log.Fatalf("错误：无法获取可执行文件路径: %v", err)
	}
	exeDir := filepath.Dir(exePath)
	log.Printf("可执行文件位于: %s", exeDir)

	// 2. 计算要服务的目录的绝对路径
	// filepath.Join 会根据操作系统自动使用正确的路径分隔符
	serveDir := filepath.Join(exeDir, *directory)
	log.Printf("尝试服务目录: %s", serveDir)

	// 3. 检查目标目录是否存在
	if _, err := os.Stat(serveDir); os.IsNotExist(err) {
		log.Printf("错误: 目录 '%s' 在可执行文件所在位置 '%s' 找不到。", *directory, exeDir)
		log.Printf("请确保名为 '%s' 的目录与此可执行文件放在同一个文件夹中。", *directory)
		// 在命令行窗口暂停，以便用户可以看到错误信息
		log.Println("按 Enter 键退出。")
		bufio.NewReader(os.Stdin).ReadBytes('\n')
		os.Exit(1) // 退出程序
	}

	// 4. 创建一个文件服务器 Handler，指向我们要服务的目录
	// http.Dir 将字符串路径转换为 http.FileSystem 类型
	fs := http.FileServer(http.Dir(serveDir))

	// 5. 应用 SPA 中间件
	// http.Handle("/", ...) 将所有请求路由到我们的处理逻辑
	// spaMiddleware 会先尝试通过 fs 提供文件，如果找不到，则返回 index.html
	http.Handle("/", spaMiddleware(serveDir, fs))

	// 6. 启动 HTTP 服务器
	listenAddr := ":" + *port
	log.Printf("正在启动服务器，服务目录 '%s' 于 http://localhost%s", serveDir, listenAddr)
	log.Println("按 Ctrl+C 停止服务器。")

	err = http.ListenAndServe(listenAddr, nil) // nil 表示使用默认的 ServeMux，我们通过 http.Handle 注册了 Handler
	if err != nil {
		log.Fatalf("错误：无法启动服务器: %v", err)
	}
}

// spaMiddleware 是一个处理单页应用 (SPA) 路由的中间件。
// 如果请求的文件在文件系统中不存在，它会返回 index.html，
// 这样 Vue Router 就可以在客户端处理路由。
func spaMiddleware(staticPath string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 排除 API 请求或其他特殊路径（如果需要）
		if strings.HasPrefix(r.URL.Path, "/api/") {
			// 如果这个服务器也需要反向代理 API，可以在这里处理
			// 但在这个场景下，我们假设 API 由另一个后端服务处理，所以这里可以返回 404
			http.NotFound(w, r)
			return
		}

		// 构建请求的文件在文件系统中的完整路径
		filePath := filepath.Join(staticPath, r.URL.Path)

		// 检查文件是否存在
		_, err := os.Stat(filePath)
		if os.IsNotExist(err) {
			// 文件不存在，很可能是一个前端路由，返回 index.html
			log.Printf("文件 '%s' 不存在, 返回 index.html", filePath)
			http.ServeFile(w, r, filepath.Join(staticPath, "index.html"))
			return
		} else if err != nil {
			// 其他读取文件状态的错误
			log.Printf("访问文件 '%s' 时出错: %v", filePath, err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// 如果文件存在，则调用默认的文件服务器 Handler (fs) 来处理
		log.Printf("服务文件: %s", filePath)
		next.ServeHTTP(w, r)
	})
}
