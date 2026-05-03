#!/bin/bash

CONTAINER_NAME="ania_sandbox"
IMAGE="alpine:latest"

# 资源限制
MEM_LIMIT="256m"
CPU_LIMIT="0.5"

if [ "$(docker ps -aq -f name=^/${CONTAINER_NAME}$)" ]; then
    echo "容器 $CONTAINER_NAME 已存在。"
    
    if [ ! "$(docker ps -q -f name=^/${CONTAINER_NAME}$)" ]; then
        echo "正在启动现有的容器..."
        docker start "$CONTAINER_NAME"
    fi
else
    echo "正在创建并启动受限容器: $CONTAINER_NAME"
    docker run -dt \
        --name "$CONTAINER_NAME" \
        --memory "$MEM_LIMIT" \
        --network host \
        --cpus "$CPU_LIMIT" \
        "$IMAGE" /bin/sh -c "sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories && apk update && /bin/sh"
    echo "资源配额已锁定: CPU: $CPU_LIMIT | MEM: $MEM_LIMIT"
fi

echo "容器运行成功，如果要进入容器，请执行"
echo "docker exec -it $CONTAINER_NAME /bin/sh"