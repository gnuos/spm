package supervisor

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"regexp"
	"sync"
)

type Project struct {
	mu sync.RWMutex

	Name     string
	WorkDir  string
	Procfile string

	running map[string]bool
}

func (p *Project) IsExist(name string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	_, ok := p.running[name]

	return ok
}

func (p *Project) GetState(name string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if _, ok := p.running[name]; ok {
		return p.running[name]
	}

	return false
}

func (p *Project) SetState(name string, state bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.running[name] = state
}

func (p *Project) GetProcNames() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	pList := make([]string, 0)

	for n := range p.running {
		pList = append(pList, n)
	}

	return pList
}

func CreateProject(opt *ProcfileOption) *Project {
	runningTab := make(map[string]bool)
	for name := range opt.Processes {
		runningTab[name] = false
	}

	return &Project{
		Name:     opt.AppName,
		WorkDir:  opt.WorkDir,
		Procfile: opt.Procfile,
		running:  runningTab,
	}
}

// GetAppName 根据当前运行的项目路径计算出一个唯一命名ID
// 借鉴了pipenv生成虚拟机环境命名的实现：
// https://github.com/pypa/pipenv/blob/main/pipenv/project.py#L524
func GetAppName(cwd string) (string, error) {
	re, err := regexp.Compile("[ &$`!*@\"()[]\\\r\n\t]")
	if err != nil {
		return "", err
	}

	fInfo, err := os.Stat(cwd)
	if err != nil {
		return "", err
	}

	cleanName := re.ReplaceAllString(fInfo.Name(), "_")
	if len(cleanName) > 42 {
		cleanName = cleanName[:42]
	}

	h := sha256.Sum256([]byte(cwd))
	hash6 := h[:6]

	encoded := base64.URLEncoding.EncodeToString(hash6)

	if len(encoded) > 8 {
		encoded = encoded[:8]
	}

	appName := fmt.Sprintf("%s-%s", cleanName, encoded)

	return appName, nil
}

// 专门设计一个应用表的结构，将读写锁的影响范围降到最小
type ProjectTable struct {
	mu sync.RWMutex

	table map[string]*Project
}

func (pt *ProjectTable) Get(name string) *Project {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	p, ok := pt.table[name]
	if ok {
		return p
	}

	return nil
}

func (pt *ProjectTable) Set(name string, p *Project) bool {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	exist := false

	if _, ok := pt.table[name]; !ok {
		exist = true
	}

	pt.table[name] = p

	return exist
}

func (pt *ProjectTable) Iter() map[string]*Project {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	return pt.table
}
