# examples

作为这个工具的示例内容

示例文件 `Procfile.options` 包含了Procfile所需的配置字段，字段存在空值时会在运行时设置默认值

代码文件 `app.py` 包含了一个Tornado项目基本完备的样板示例，可以用于生产借鉴


### 使用方法

首先推荐安装uv，便于快速体验工具的用法

主流的Linux发行版都在软件仓库里添加了uv的包，如果发行版的软件仓库中没有找到，就执行下面的命令：

`curl -LsSf https://astral.sh/uv/install.sh | sh`


安装完成之后，在当前目录里面执行下面的命令，用来初始化虚拟环境

```bash
uv sync

```

直接执行编译好的工具，可以看到能够使用的子命令列表：

`../bin/spm help`


首次执行 start 会默认使用子进程的方式启动一个后台的守护进程


