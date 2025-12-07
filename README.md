# Spm

本工具的理念是简单快速管理后台进程。基本是 [Procodile](https://github.com/adamcooke/procodile) 这个项目的移植版，procodile 这个项目已经没有再维护了，大部分代码都是参考了这个项目的Ruby代码用Go改写的。

主要的特点是根据Procfile格式批量管理应用进程，另外也可以支持运行多个不同目录里的项目进程。

当前也支持从命令行接收一行命令和参数作为后台进程运行，运行进程的工作目录就是执行命令的目录位置。


## 安装方法

在Linux系统环境执行下面的命令：

```bash
git clone https://github.com/gnuos/spm
cd spm
just

```

如果没有安装 `just` 这个工具，可以手动执行下面的命令进行编译：

`go build -ldflags="-w -s -extldflags '-static'" -o ./bin/spm .`


编译得到的 `bin/spm` 文件就是最终的单个二进制可执行文件，可以放在其他的PATH路径中便于使用。


## 使用方法

在任意一个目录下面，放一个Procfile，参考 [The Procfile](https://devcenter.heroku.com/articles/procfile) 文章中的格式填写。

进入到存在 Procfile 文件的目录中，或者在命令参数里指定 Procfile 文件的位置和运行时的工作目录，就可以把项目运行到后台了。


执行后的效果如下所示：

```bash
$ ./bin/spm
Usage:
  spm [flags]
  spm [command]

Available Commands:
  daemon      Run supervisor as a daemon
  help        Help about any command
  reload      Reload processes and options
  restart     Restart processes
  run         Run command as a process
  shutdown    Stop supervisor
  start       Starts processes and/or the supervisor
  status      Check processed status
  stop        Stop processes
  version     Print version and exit

Flags:
  -h, --help              help for spm
  -l, --loglevel string   Set log Level (default "debug")
  -p, --procfile string   The path to the Procfile (default "/opt/spm/Procfile")
  -v, --version           Print version and exit
  -w, --workdir string    The path to the work directory (default "/opt/spm")

Use "spm [command] --help" for more information about a command.

```


在项目的 `example` 目录中，可以看到示例文件，以供参考。


## 致谢

感谢以下项目提供的代码参考

- [mikestefanello/pagoda](https://github.com/mikestefanello/pagoda) 借鉴了加载配置文件的写法
- [kondohiroki/go-boilerplate](https://github.com/kondohiroki/go-boilerplate) 参考了日志和配置文件的结构
- [codoworks/go-boilerplate](https://github.com/codoworks/go-boilerplate) 参考了子命令的写法
- [cskr/pubsub](https://github.com/cskr/pubsub) 抄写了消息队列的实现


## License

Spm is licensed under the MIT license.

See LICENSE for the full license text.

