package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

func main() {

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

	workdir := filepath.Dir(execPath)
	defaultBackupPath := fmt.Sprintf("%s/backups", workdir)
	defaultServerFilesPath := workdir

	fmt.Println(defaultBackupPath)

	listenport := flag.Int("lp", 7777, "api server listen port")
	backuppath := flag.String("backups", defaultBackupPath, "path where server backups are stored\n\nIMPORTANT: any parent directory specified must exist.\nExample:\n  - if you want to store backups in /etc/minecraft/backups, directory /etc/minecraft MUST EXIST\n")
	serverfiles := flag.String("server-files", defaultServerFilesPath, "path where server files are stored\n\nIMPORTANT: any parent directory specified must exist.\nExample:\n  - if you want to store backups in /etc/minecraft/serverfiles, directory /etc/minecraft MUST EXIST\n")
	trustedpath := flag.String("trusted-path", trustedPath, "boundary path to which user can traverse specifying backup/server directories")

	flag.Parse()

	// server params
	listenPort := fmt.Sprintf(":%d", *listenport)
	backupPath := filepath.Clean(*backuppath)
	serverFilesPath := filepath.Clean(*serverfiles)
	trustedPath = *trustedpath

	fmt.Println(listenPort, backupPath, serverFilesPath)
	backupPath, err = verifyPath(trustedPath, backupPath)
	if err != nil {
		log.Fatalln("invalid backup path: ", backupPath, "error: ", err)
	}

	serverFilesPath, err = verifyPath(trustedPath, serverFilesPath)
	if err != nil {
		log.Fatalln("invalid bind path: ", serverFilesPath, "error: ", err)
	}

	if serverFilesPath == backupPath {
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
	runner := InitRunner(img, cn, serverFilesPath)

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
