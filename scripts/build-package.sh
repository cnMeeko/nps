#!/usr/bin/env bash
# 构建单个平台的 NPS 服务端 / 客户端安装包
# 用法：build-package.sh <GOOS> <GOARCH> <GOARM> <包名> [<可执行文件后缀>]
# 示例：build-package.sh linux amd64 "" linux_amd64
#       build-package.sh windows amd64 "" windows_amd64 .exe
#       build-package.sh linux arm 7 linux_arm_v7
set -euo pipefail

GOOS_IN=${1:?GOOS 不能为空}
GOARCH_IN=${2:?GOARCH 不能为空}
GOARM_IN=${3:-}
NAME=${4:?包名不能为空}
EXT=${5:-}

export CGO_ENABLED=0
export GOOS="$GOOS_IN"
export GOARCH="$GOARCH_IN"
if [ -n "$GOARM_IN" ]; then
  export GOARM="$GOARM_IN"
fi

WORK_DIR="dist/build/$NAME"
mkdir -p "$WORK_DIR" release

# 编译服务端与客户端
go build -trimpath -ldflags "-s -w" -o "$WORK_DIR/nps$EXT" ./cmd/nps
go build -trimpath -ldflags "-s -w" -o "$WORK_DIR/npc$EXT" ./cmd/npc

# 服务端：可执行文件 + conf + web/views + web/static
SERVER_DIR="$WORK_DIR/server"
rm -rf "$SERVER_DIR"
mkdir -p "$SERVER_DIR/web"
cp "$WORK_DIR/nps$EXT" "$SERVER_DIR/"
cp -r conf "$SERVER_DIR/conf"
cp -r web/views "$SERVER_DIR/web/views"
cp -r web/static "$SERVER_DIR/web/static"
tar -czf "release/${NAME}_server.tar.gz" -C "$SERVER_DIR" .

# 客户端：可执行文件 + conf/npc.conf + conf/multi_account.conf
CLIENT_DIR="$WORK_DIR/client"
rm -rf "$CLIENT_DIR"
mkdir -p "$CLIENT_DIR/conf"
cp "$WORK_DIR/npc$EXT" "$CLIENT_DIR/"
cp conf/npc.conf conf/multi_account.conf "$CLIENT_DIR/conf/"
tar -czf "release/${NAME}_client.tar.gz" -C "$CLIENT_DIR" .

echo "已生成 release/${NAME}_server.tar.gz release/${NAME}_client.tar.gz"
