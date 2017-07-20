#include "sck.h"
#include "redo.h"

/******************************************************************************
 **函数名称: tcp_listen
 **功    能: 侦听指定端口
 **输入参数:
 **     port: 端口号
 **输出参数: NONE
 **返    回: 套接字ID
 **实现描述:
 **     1. 创建套接字
 **     2. 绑定指定端口
 **     3. 侦听指定端口
 **     4. 设置套接字属性(可重用、非阻塞)
 **注意事项:
 **     1. LINGER属性: 默认情况下都是关闭的, 也不建议开启此选项.
 **        struct linger {
 **            int l_onoff;  // LINGER属性开关(0:关闭 !0:开启)
 **            int l_linger; // 最大超时时间
 **        }
 **      > 如果l_onoff = 0, 那么关闭本选项. 当调用close()函数会立即返回. 如果缓
 **        冲区有数据，系统将会试着把数据发送给对端.
 **      > 如果l_onoff != 0, 且l_linger > 0时, 无论套接字是"阻塞"还是"非阻塞",
 **        当调用close()函数时. 只要未收到对端FIN应答, 则会导致close()函数阻塞,
 **        最大阻塞时间为l_linger.
 **      > 如果l_onoff != 0, 且l_linger = 0时, 当调用close()时TCP将丢弃套接字缓
 **        冲区中的所有数据, 并发送RST给对端. 这么一来避免了TIME_WAIT状态. 然而
 **        这会带来另外一个问题: 在2MSL秒内创建该连接的另外一个化身, 将导致来自
 **        刚被终止的链接上的旧的重复分节被不正确的传送到新的化身上.
 **作    者: # Qifeng.zou # 2014.03.24 #
 ******************************************************************************/
int tcp_listen(int port)
{
    int fd, opt = 1;
    struct sockaddr_in svraddr;

    /* 1. 创建套接字 */
    fd = socket(AF_INET, SOCK_STREAM, 0);
    if (fd < 0) {
        return -1;
    }

    opt = 1;
    setsockopt(fd, SOL_SOCKET, SO_REUSEADDR, &opt, sizeof(int));

    /* 2. 绑定指定端口 */
    bzero(&svraddr, sizeof(svraddr));

    svraddr.sin_family = AF_INET;
    svraddr.sin_addr.s_addr = htonl(INADDR_ANY);
    svraddr.sin_port = htons(port);

    if (bind(fd, (struct sockaddr *)&svraddr, sizeof(svraddr)) < 0) {
        close(fd);
        return -1;
    }

    /* 3. 侦听指定端口 */
    if (listen(fd, TCP_LISTEN_BACKLOG) < 0) {
        close(fd);
        return -1;
    }

    /* 4. 设置非阻塞属性 */
    fd_set_nonblocking(fd);

    return fd;
}

/******************************************************************************
 **函数名称: tcp_accept
 **功    能: 接收连接请求
 **输入参数:
 **     lsnfd: 侦听套接字
 **输出参数:
 **     cliaddr: 客户端信息
 **返    回: 套接字ID
 **实现描述:
 **     1. 接收连接请求
 **     2. 设置套接字属性
 **注意事项:
 **作    者: # Qifeng.zou # 2015.04.07 #
 ******************************************************************************/
int tcp_accept(int lsnfd, struct sockaddr *cliaddr)
{
    int fd, opt;
    socklen_t len = sizeof(cliaddr);

    /* > 接收连接请求 */
    fd = accept(lsnfd, (struct sockaddr *)cliaddr, &len);
    if (fd < 0) {
        return -1;
    }

    /* > 设置套接字属性 */
    opt = 1;
    setsockopt(fd, SOL_SOCKET, SO_REUSEADDR, &opt, sizeof(int)); /* 解决: 端口被占用的问题 */

    fd_set_nonblocking(fd);

    return fd;
}

/******************************************************************************
 **函数名称: tcp_connect
 **功    能: 连接指定服务器
 **输入参数:
 **     family: 协议(AF_INET/AF_INET6)
 **     ipaddr: IP地址
 **     port: 端口号
 **输出参数: NONE
 **返    回: 套接字ID
 **实现描述:
 **     1. 创建套接字
 **     2. 连接指定服务器
 **     3. 设置套接字属性(非阻塞)
 **注意事项:
 **     SO_REUSEADDR可以用在以下四种情况下:
 **         (摘自《Unix网络编程》卷一, 即UNPv1)
 **         1) 当有一个有相同本地地址和端口的socket1处于TIME_WAIT状态时, 而你启
 **     动的程序的socket2要占用该地址和端口, 你的程序就要用到该选项.
 **         2) SO_REUSEADDR允许同一port上启动同一服务器的多个实例(多个进程). 但
 **     每个实例绑定的IP地址是不能相同的. 在有多块网卡或用IP Alias技术的机器可
 **     以测试这种情况.
 **         3) SO_REUSEADDR允许单个进程绑定相同的端口到多个socket上，但每个soc
 **     ket绑定的ip地址不同。这和2很相似，区别请看UNPv1。
 **         4) SO_REUSEADDR允许完全相同的地址和端口的重复绑定。但这只用于UDP的
 **     多播，不用于TCP。
 **作    者: # Qifeng.zou # 2014.09.24 #
 ******************************************************************************/
int tcp_connect(int family, const char *ipaddr, int port)
{
    int fd, opt = 1;
    struct linger lg;
    struct sockaddr_in svraddr;

    /* 1. 创建套接字 */
    fd = socket(family, SOCK_STREAM, 0);
    if (fd < 0) {
        return -1;
    }

    opt = 1;
    setsockopt(fd, SOL_SOCKET, SO_REUSEADDR, &opt, sizeof(int));

    /* 有效避免TIME_WAIT过多造成的问题 */
    memset(&lg, 0, sizeof(lg));

    lg.l_onoff = 1;
    lg.l_linger = 0;
    setsockopt(fd, SOL_SOCKET, SO_LINGER, &lg, sizeof(lg));

    /* 2. 连接远程服务器 */
    bzero(&svraddr, sizeof(svraddr));

    svraddr.sin_family = AF_INET;
    inet_pton(AF_INET, ipaddr, &svraddr.sin_addr);
    svraddr.sin_port = htons(port);

    if (0 != connect(fd, (struct sockaddr *)&svraddr, sizeof(svraddr))) {
        close(fd);
        return -1;
    }

    fd_set_nonblocking(fd);

    return fd;
}

/******************************************************************************
 **函数名称: tcp_connect_timeout
 **功    能: 连接指定服务器(超时返回)
 **输入参数:
 **     family: 协议(AF_INET/AF_INET6)
 **     ipaddr: IP地址
 **     port: 端口号
 **     sec: 超时时间
 **输出参数: NONE
 **返    回: 套接字ID
 **实现描述:
 **     1. 创建套接字
 **     2. 连接指定服务器
 **     3. 设置套接字属性(非阻塞)
 **注意事项:
 **     1) SO_REUSEADDR:
 **作    者: # Qifeng.zou & Menglai.Wang # 2014.09.25 #
 ******************************************************************************/
int tcp_connect_timeout(int family, const char *ipaddr, int port, int sec)
{
    struct linger lg;
    int ret, fd, opt = 1;
    fd_set rdset, wrset;
    struct timeval tv;
    struct sockaddr_in svraddr;

    /* 1. 创建套接字 */
    fd = socket(family, SOCK_STREAM, 0);
    if (fd < 0) {
        return -1;
    }

    opt = 1;
    setsockopt(fd, SOL_SOCKET, SO_REUSEADDR, &opt, sizeof(int));

    /* 有效避免TIME_WAIT过多造成的问题 */
    memset(&lg, 0, sizeof(lg));

    lg.l_onoff = 1;
    lg.l_linger = 0;
    setsockopt(fd, SOL_SOCKET, SO_LINGER, &lg, sizeof(lg));

    fd_set_nonblocking(fd);

    /* 2. 连接远程服务器 */
    bzero(&svraddr, sizeof(svraddr));

    svraddr.sin_family = AF_INET;
    inet_pton(AF_INET, ipaddr, &svraddr.sin_addr);
    svraddr.sin_port = htons(port);

    if (!connect(fd, (struct sockaddr *)&svraddr, sizeof(svraddr))) {
        return fd;
    }

    if (EINPROGRESS != errno) {
        close(fd);
        return -1;
    }

    /* 3. 判断是否超时 */
AGAIN:
    FD_ZERO(&rdset);
    FD_ZERO(&wrset);

    FD_SET(fd, &rdset);
    FD_SET(fd, &wrset);

    tv.tv_sec = sec;
    tv.tv_usec = 0;
    ret = select(fd+1, &rdset, &wrset, NULL, &tv);
    if (ret < 0) {
        if (EINTR == errno) { goto AGAIN; }
        close(fd);
        return -1;
    } else if (0 == ret) {
        close(fd);
        return -1;
    }

    return fd;
}

/******************************************************************************
 **函数名称: tcp_connect_async
 **功    能: 连接指定服务器
 **输入参数:
 **     family: 协议(AF_INET/AF_INET6)
 **     ipaddr: IP地址
 **     port: 端口号
 **输出参数: NONE
 **返    回: 套接字ID
 **实现描述:
 **     1. 创建套接字
 **     2. 设置为非阻塞套接字
 **     3. 连接指定服务器
 **注意事项:
 **作    者: # Qifeng.zou # 2014.10.09 #
 ******************************************************************************/
int tcp_connect_async(int family, const char *ipaddr, int port)
{
    int fd, opt = 1, ret;
    struct linger lg;
    struct sockaddr_in svraddr;

    /* 1. 创建套接字 */
    fd = socket(family, SOCK_STREAM, 0);
    if (fd < 0) {
        return -1;
    }

    opt = 1;
    setsockopt(fd, SOL_SOCKET, SO_REUSEADDR, &opt, sizeof(int));

    /* 有效避免TIME_WAIT过多造成的问题 */
    memset(&lg, 0, sizeof(lg));

    lg.l_onoff = 1;
    lg.l_linger = 0;
    setsockopt(fd, SOL_SOCKET, SO_LINGER, &lg, sizeof(lg));

    fd_set_nonblocking(fd);

    /* 2. 连接远程服务器 */
    bzero(&svraddr, sizeof(svraddr));

    svraddr.sin_family = AF_INET;
    inet_pton(AF_INET, ipaddr, &svraddr.sin_addr);
    svraddr.sin_port = htons(port);

    ret = connect(fd, (struct sockaddr *)&svraddr, sizeof(svraddr));
    if ((0 == ret) || (EINPROGRESS == errno)) {
        return fd;
    }

    close(fd);
    return -1;
}

/******************************************************************************
 **函数名称: fd_is_writable
 **功    能: 判断文件描述符是否可写
 **输入参数:
 **     fd: 文件描述符
 **输出参数: NONE
 **返    回: 0:否 !0:是
 **实现描述:
 **注意事项:
 **作    者: # Qifeng.zou # 2014.06.07 #
 ******************************************************************************/
int fd_is_writable(int fd)
{
    fd_set wset;
    struct timeval tv;

    FD_ZERO(&wset);
    FD_SET(fd, &wset);

    tv.tv_sec = 0;
    tv.tv_usec = 0;

    return select(fd+1, NULL, &wset, NULL, &tv);
}

/******************************************************************************
 **函数名称: tcp_block_send
 **功    能: 阻塞发送
 **输入参数:
 **    fd: 文件描述符
 **    addr: 被发送数据的起始地址
 **    len: 发送字节数
 **输出参数: NONE
 **返    回: 0:Succ !0:Fail
 **实现描述:
 **注意事项: 等待所有字节发送完成后才返回，除非出现严重错误.
 **作    者: # Qifeng.zou # 2014.06.24 #
 ******************************************************************************/
int tcp_block_send(int fd, const void *addr, int len, int timeout)
{
    int ret, n, left = len, off = 0;
    fd_set wrset;
    struct timeval tv;

    for (;;) {
        FD_ZERO(&wrset);
        FD_SET(fd, &wrset);

        tv.tv_sec = timeout;
        tv.tv_usec = 0;
        ret = select(fd+1, NULL, &wrset, NULL, &tv);
        if (ret < 0) {
            if (EINTR == errno) { continue; }
            return -1;
        } else if (0 == ret) {
            return -1;
        }

        n = Writen(fd, (const char *)addr + off, left);
        if (n < 0) {
            return -1;
        } else if (left != n) {
            left -= n;
            off += n;
            continue;
        }

        return 0; /* Done */
    }
}

/******************************************************************************
 **函数名称: tcp_block_recv
 **功    能: 阻塞接收
 **输入参数:
 **     fd: 文件描述符
 **     addr: 被发送数据的起始地址
 **     len: 发送字节数
 **     timeout: 阻塞时长(秒)
 **输出参数: NONE
 **返    回: 0:Succ !0:Fail
 **实现描述:
 **注意事项: 等待所有字节接收完成后才返回，除非出现严重错误或超时.
 **作    者: # Qifeng.zou # 2014.06.24 #
 ******************************************************************************/
int tcp_block_recv(int fd, void *addr, int len, int timeout)
{
    int ret, n, left = len, off = 0;
    fd_set rdset;
    struct timeval tv;

    for (;;) {
        FD_ZERO(&rdset);
        FD_SET(fd, &rdset);

        tv.tv_sec = timeout;
        tv.tv_usec = 0;
        ret = select(fd+1, &rdset, NULL, NULL, &tv);
        if (ret < 0) {
            if (EINTR == errno) { continue; }
            return -1;
        } else if (0 == ret) {
            return -1;
        }

        n = Readn(fd, (char *)addr + off, left);
        if (n < 0) {
            return -1;
        } else if (left != n) {
            left -= n;
            off += n;
            continue;
        }

        return 0; /* Done */
    }
}

/******************************************************************************
 **函数名称: ip_isvalid
 **功    能: 判断IP地址的合法性(是否符合点分十进制的格式)
 **输入参数:
 **     ip: IP地址
 **输出参数: NONE
 **返    回: true:合法 false:非法
 **实现描述:
 **注意事项:
 **作    者: # Qifeng.zou # 2015.03.02 #
 ******************************************************************************/
bool ip_isvalid(const char *ip)
{
    int dots = 0, digits;
    const char *p = ip;

    while (' ' == *p) { ++p; }

    digits = 0;
    while ('\0' != *p) {
        while(isdigit(*p)) {
            ++p;
            ++digits;
        }

        if ((digits < 1) || (digits > 3)) {
            return false;
        }

        if ('.' == *p) {
            ++dots;
            if (dots > 3) {
                return false;
            }
            ++p;
            digits = 0;
            continue;
        } else if ('\0' == *p) {
            if (3 != dots) {
                return false;
            }
            return true;
        } else if (' ' == *p) {
            while (' ' == *p) { ++p; };
            if ('\0' != *p) {
                return false;
            }
            return true;
        }

        return false;
    }

    if ((3 != dots)
        || (0 == digits)
        || (digits > 3))
    {
        return false;
    }

    return true;
}

/******************************************************************************
 **函数名称: iplist_parse
 **功    能: 解析IP列表
 **输入参数:
 **     iplist: IP列表字串
 **输出参数:
 **     list: IP列表
 **返    回: 0:成功 !0:失败
 **实现描述: 按照字串格式解析&提取数据
 **注意事项: 格式:${IP1}:${PORT1},${IP2}:${PORT2},${IP...}:${PORT...}
 **作    者: # Qifeng.zou # 2017.07.19 18:18:15 #
 ******************************************************************************/
list_t *iplist_parse(const char *iplist)
{
    list_t *list;
    int digit, len, idx;
    iplist_item_t *item;
    const char *ptr = iplist, *start, *end;

    /* > 创建列表对象 */
    list = list_creat(NULL);
    if (NULL == list) {
        return NULL;
    }

    /* > 解析&提取IP列表 */
    idx = 0;
    while (1) {
        while (' ' == *ptr || ',' == *ptr) {
            ptr += 1;
        }

        if ('\0' == *ptr) {
            break;
        }

        start = ptr;

        /* 第一个字段 */
        digit = 0;
        while (isdigit(*ptr)) {
            digit = 10 * digit + (*ptr - '0');
            ++ptr;
        }

        if (('.' != *ptr) || (0 == digit || digit > 255)) {
            goto ERR_IPLIST;
        }

        /* 第二个字段 */
        ++ptr;
        digit = 0;
        while (isdigit(*ptr)) {
            digit = 10 * digit + (*ptr - '0');
            ++ptr;
        }

        if (('.' != *ptr) || (digit > 255)) {
            goto ERR_IPLIST;
        }

        /* 第三个字段 */
        ++ptr;
        digit = 0;
        while (isdigit(*ptr)) {
            digit = 10 * digit + (*ptr - '0');
            ++ptr;
        }

        if (('.' != *ptr) || (digit > 255)) {
            goto ERR_IPLIST;
        }

        /* 第四个字段 */
        ++ptr;
        digit = 0;
        while (isdigit(*ptr)) {
            digit = 10 * digit + (*ptr - '0');
            ++ptr;
        }

        if ((':' != *ptr) || (0 == digit || digit > 255)) {
            goto ERR_IPLIST;
        }

        end = ptr; /* 注意: 此时end指向字符':'冒号 */

        /* 端口号 */
        ++ptr;
        digit = 0;
        while (isdigit(*ptr)) {
            digit = 10 * digit + (*ptr - '0');
            ++ptr;
        }

        if (0 == digit || digit > 65535) {
            goto ERR_IPLIST;
        }

        /* 存储数据 */
        item = (iplist_item_t *)calloc(1, sizeof(iplist_item_t));
        if (NULL == item) {
            goto ERR_IPLIST;
        }

        len = end - start;
        if (len >= (int)(sizeof(item->ipaddr) - 1)) {
            free(item);
            goto ERR_IPLIST;
        }

        memcpy(item->ipaddr, start, len);
        item->port = digit;
        item->idx = idx++;

        list_rpush(list, item);
    }

    if (0 == list_length(list)) {
        goto ERR_IPLIST;
    }

    return list;

ERR_IPLIST:
    list_destroy(list, (mem_dealloc_cb_t)mem_dealloc, NULL);
    return NULL;
}
