package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/common-nighthawk/go-figure"
)

type BindPath struct {
	Path  string
	Label string
}

func main() {

	fmt.Println("================================================================================================================")
	// fig1 := figure.NewFigure("welcome to:", "larry3d", true)
	// fig1.Print()
	fig2 := figure.NewFigure("mcmgmt-api", "larry3d", true)
	fig2.Print()
	fmt.Println("================================================================================================================")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage of %s:\n", os.Args[0])
		flag.PrintDefaults() // prints the default flags
		os.Exit(0)           // exit after showing help
	}

	execPath, err := os.Executable()
	if err != nil {
		log.Fatalln("cannot fetch current directory: ", err)
	}
	trustedPath, err := os.UserHomeDir()
	if err != nil {
		log.Fatalln("cannot fetch user home directory: ", err)
	}

	workdir := filepath.Dir(execPath)                       // working directory  (.)
	defaultBackupPath := fmt.Sprintf("%s/backups", workdir) // folder in working directory (./backups)
	defaultServerFilesPath := workdir                       // working directory (.)
	defaultMemory := 4                                      // in gigabytes

	listenport := flag.Int("lp", 7777, "api server listen port")
	backuppath := flag.String("backups", defaultBackupPath, "path where server backups are stored\n\nIMPORTANT: any parent directory specified must exist.\nExample:\n  - if you want to store backups in /etc/minecraft/backups, directory /etc/minecraft MUST EXIST\n")
	serverfiles := flag.String("server-files", defaultServerFilesPath, "path where server files are stored\n\nIMPORTANT: any parent directory specified must exist.\nExample:\n  - if you want to store backups in /etc/minecraft/serverfiles, directory /etc/minecraft MUST EXIST\n")
	trustedpath := flag.String("trusted-path", trustedPath, "boundary path to which user can traverse specifying backup/server directories")
	memory := flag.Int64("memory", int64(defaultMemory), "memory for the minecraft server in GB")

	flag.Parse()

	log.Printf("Started mcmgmt-api binary with PID %v\n", os.Getpid())
	log.Println("default backup path: ", defaultBackupPath)
	log.Println("default server files path: ", defaultServerFilesPath)
	log.Println("default memory path: ", defaultMemory)
	log.Println("default trusted path: ", trustedPath)

	// server params
	listenPort := fmt.Sprintf(":%d", *listenport)
	backupBindPath := BindPath{
		Path:  filepath.Clean(*backuppath),
		Label: "backup",
	}
	serverFilesBindPath := BindPath{
		Path:  filepath.Clean(*serverfiles),
		Label: "serverfiles",
	}

	trustedPath = *trustedpath

	// fmt.Println(listenPort, backupBindPath, serverFilesBindPath)
	backupPath, err := verifyPath(trustedPath, backupBindPath)
	if err != nil {
		log.Fatalln("invalid backup path: ", backupBindPath, "error: ", err)
	}

	serverFilesPath, err := verifyPath(trustedPath, serverFilesBindPath)
	if err != nil {
		log.Fatalln("invalid bind path: ", serverFilesBindPath, "error: ", err)
	}

	if serverFilesPath == backupBindPath.Path {
		log.Fatalln("backup and server files paths cannot be the same")
	}

	// container params
	img := "itzg/minecraft-server"
	cn := fmt.Sprintf("mcserver-%s", randomString(9))

	// needed envs
	secret := os.Getenv("JWT_SECRET")
	bucketName := os.Getenv("BACKUPS_BUCKET")
	projectID := os.Getenv("PROJECT_ID")

	//init server

	logPath := fmt.Sprintf("%v/mcdata/logs/latest.log", serverFilesPath)

	// create login service
	loginSvc := NewLoginService(secret)

	// create runner
	runner := InitRunner(img, cn, serverFilesPath, *memory)

	// create bucket controller
	bucket, err := InitBucket(bucketName, projectID, backupPath)
	if err != nil {
		log.Fatalln(err)
	}

	backupSvc := NewBackupService(bucket, backupPath)

	// create API server instance
	server := NewAPIServer(listenPort, logPath, loginSvc, runner, backupSvc, secret)
	server.Run()
}
