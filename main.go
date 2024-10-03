package main

import (
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/polly"
	"github.com/spf13/viper"
	"github.com/zaf/agi"
)

// Polly
type ContollerPolly struct {
	svc *polly.Polly
	agi *agi.Session
}

func NewControllerPolly(agiloc *agi.Session) ContollerPolly {
	var controller ContollerPolly

	// Tente inicializar a sessão AWS
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))
	controller.agi = agiloc
	controller.agi.Verbose("polly controller")
	if sess == nil {
		controller.agi.Verbose("Erro: sessão AWS Polly não foi criada!")
	} else {
		controller.agi.Verbose("Sessão AWS Polly criada com sucesso")
	}

	controller.svc = polly.New(sess)
	return controller
}

func (c *ContollerPolly) RequestAudio(texto string, voice string) (*polly.SynthesizeSpeechOutput, error) {
	// Configure os parâmetros da solicitação
	input := &polly.SynthesizeSpeechInput{
		Engine:       aws.String("standard"),
		OutputFormat: aws.String("mp3"),
		SampleRate:   aws.String("8000"),
		Text:         aws.String(texto),
		VoiceId:      aws.String(voice),
		LanguageCode: aws.String("pt-BR"),
	}

	// Faça a solicitação de sintetização de fala
	c.agi.Verbose("solicitaod audio a polly")
	resp, err := c.svc.SynthesizeSpeech(input)
	if err != nil {
		c.agi.Verbose(fmt.Sprint("Erro ao solicitar a síntese de fala: ", err))
		return nil, err
	}

	return resp, nil
}

type Config struct {
	Dir          string
	FilePrefix   string
	FileExten    string
	DefaultVoice string
	Engine       string
	Loglevel     int
}

// Main mas voltando um int para ser usado como ExitCode
func ExecMain() int {
	// set envs e logs UP
	var config Config

	pwd := "/var/lib/asterisk/"
	os.Setenv("HOME", pwd)

	viper.AddConfigPath(pwd)
	viper.SetConfigName("app")
	viper.SetConfigType("env")
	viper.AutomaticEnv()
	err := viper.ReadInConfig()
	err = viper.Unmarshal(&config)
	if err != nil {
		fmt.Println("Configurações não carregadas: ", err)
		return 3
	}
	if config.Dir == "" {
		fmt.Println("Configurações não carregadas corretamente")
		return 3
	}
	Agi := agi.New()
	erragi := Agi.Init(nil)
	if erragi != nil {
		fmt.Println("falha ao carregar agi")
	}

	var dir string = config.Dir
	var file string = config.FilePrefix
	var ext string

	if config.FileExten == "" {
		ext = ".wav"
	} else {
		ext = config.FileExten
	}

	Agi.Verbose("inicio do script")
	if _, err := os.ReadDir(dir); err != nil {
		os.Mkdir(dir, 0775)
	}

	// Defina o texto que deseja sintetizar
	Agi.Verbose("start args")
	var texto string = os.Args[1]
	var voice string = os.Args[2]
	vocalize, _ := strconv.ParseBool(os.Args[3])

	if len(texto) <= 0 {
		Agi.Verbose("Texto Vazio:")
		return 1
	}
	if len(voice) <= 0 {
		voice = config.DefaultVoice
	}

	// instanciamento das demais variaves de dir
	textoMD5 := md5.Sum([]byte(texto))
	file += fmt.Sprintf("%x", textoMD5)
	var dirFile string = dir + file
	var tmpFile string = "/tmp/" + file + ".mp3"

	if _, err := os.ReadFile(dirFile + ext); err == nil {
		Agi.Verbose("Arquivo " + file + " ja existe no servidor")
		if vocalize {
			rpl, _ := Agi.GetData(dirFile, 1, 10)
			if rpl.Res != 0 {
				Agi.SetExtension(strconv.Itoa(rpl.Res))
				Agi.SetPriority("1")
			}
		}
		return 0
	}

	Agi.Verbose("iniciar polly")
	// Instancia o polly e faz o request
	plly := NewControllerPolly(Agi)
	resp, err := plly.RequestAudio(texto, voice)
	if err != nil {
		Agi.Verbose("Erro ao criar construir polly:", 1)
		return 4
	}

	Agi.Verbose("audio solicitado")
	// Salve o áudio de saída em um arquivo
	outFile, err := os.Create(tmpFile)
	if err != nil {
		Agi.Verbose("Erro ao criar o arquivo de saída:", 1)
		return 1
	}

	// Copie os dados de áudio da resposta para o arquivo de saída
	if _, err := io.Copy(outFile, resp.AudioStream); err != nil {
		Agi.Verbose("Erro ao copiar os dados de áudio para o arquivo de saída")
		return 1
	}
	defer outFile.Close()

	// Converte o arquivo pro formato certo
	err = soxConvert(tmpFile, dirFile+ext)
	if err != nil {
		Agi.Verbose(fmt.Sprintf("erro na conversao do arquivo: %v", err))
		return 12
	}
	// verbose do arquvo criado
	Agi.Verbose("antes de ler o arquivo")
	if _, errd := os.ReadFile(dirFile + ext); errd == nil {
		Agi.Verbose("Arquivo" + file + "Criado com sucesso!")
		if vocalize {
			rpl, _ := Agi.GetData(dirFile, 1, 10)
			if rpl.Res != 0 {
				Agi.SetExtension(strconv.Itoa(rpl.Res))
			}
		}
		Agi.Verbose("Síntese de fala concluída com sucesso. Áudio salvo em " + dirFile + ext)
		return 0
	} else {
		Agi.Verbose("Aquivo não pode ser localizado, gerado incorretamente" + errd.Error())
		return 1
	}
}

// TODO  refazer esta func  pra usar sox do golang
func soxConvert(infile, outfile string) error {

	var Path string = "/usr/bin/sox"
	var Args = []string{"--ignore-length", infile, "-r 8000", "-c 1", outfile}

	command := exec.Command(Path, Args...)
	_, err := command.Output()
	if err != nil {
		return err
	}

	return nil
}

func main() { os.Exit(ExecMain()) }
