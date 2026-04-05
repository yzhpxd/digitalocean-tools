package main

import (
	"bufio"
	"context"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/digitalocean/godo"
	"golang.org/x/oauth2"
)

var (
	client *godo.Client
	ctx    = context.TODO()
	// 汉化映射表
	regionCN = map[string]string{
		"nyc1": "纽约 1", "nyc2": "纽约 2", "nyc3": "纽约 3",
		"sfo2": "旧金山 2", "sfo3": "旧金山 3",
		"sgp1": "新加坡", "lon1": "伦敦", "fra1": "法兰克福",
		"ams3": "阿姆斯特丹", "tor1": "多伦多", "blr1": "班加罗尔",
		"syd1": "悉尼", "atl1": "亚特兰大", "ric1": "里士满",
	}
)

func main() {
	rand.Seed(time.Now().UnixNano())

	fmt.Println("========================================")
	fmt.Println("    DigitalOcean 全能定制助手 v9.0")
	fmt.Println("  (支持: 选硬盘/选CPU/自定义初始脚本)")
	fmt.Println("========================================")

	var token string
	for {
		fmt.Print("\n请输入您的 API Token: ")
		fmt.Scanln(&token)
		if len(token) > 20 {
			break
		}
		fmt.Println("⚠️ Token 格式不正确。")
	}

	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	client = godo.NewClient(oauth2.NewClient(ctx, ts))

	_, _, err := client.Account.Get(ctx)
	if err != nil {
		fmt.Printf("❌ 认证失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✅ 认证成功！")

	for {
		showMainMenu()
	}
}

func showMainMenu() {
	fmt.Println("\n--------- [ 主 菜 单 ] ---------")
	fmt.Println("1. 查看并管理机器 (含无损换IP)")
	fmt.Println("2. 全定制新建机器 (自选配置/硬盘/系统/脚本)")
	fmt.Println("3. 退出程序")
	fmt.Print("请选择 (1-3): ")

	var choice string
	fmt.Scanln(&choice)
	switch choice {
	case "1":
		listAndManage()
	case "2":
		customCreate()
	case "3":
		os.Exit(0)
	default:
		fmt.Println("❌ 无效选择")
	}
}

// ==================== 1. 列表与管理模块 ====================

func listAndManage() {
	list, _, err := client.Droplets.List(ctx, &godo.ListOptions{PerPage: 100})
	if err != nil {
		fmt.Printf("获取失败: %v\n", err)
		return
	}
	if len(list) == 0 {
		fmt.Println("\n📭 账号下暂无机器。")
		return
	}

	fmt.Printf("\n%-4s %-10s %-20s %-18s %-15s %-8s\n", "序号", "ID", "名称", "IPv4 (原生/保留)", "IPv6", "状态")
	fmt.Println("------------------------------------------------------------------------------------------------")

	dropletMap := make(map[int]godo.Droplet)
	for i, d := range list {
		idx := i + 1
		dropletMap[idx] = d

		var v4, resIP, v6 string
		v6 = "---"
		for _, n := range d.Networks.V4 {
			if n.Type == "public" { v4 = n.IPAddress }
			if n.Type == "reserved" { resIP = n.IPAddress }
		}
		for _, n := range d.Networks.V6 {
			if n.Type == "public" { v6 = n.IPAddress }
		}

		displayV4 := v4
		if resIP != "" { displayV4 = fmt.Sprintf("%s*", resIP) }

		fmt.Printf("[%d]  %-10d %-20s %-18s %-15s %-8s\n", idx, d.ID, d.Name, displayV4, v6, d.Status)
	}
	fmt.Println("------------------------------------------------------------------------------------------------")
	fmt.Println("提示: IPv4 带 * 号表示当前正在使用 [保留IP]")

	fmt.Print("\n请输入 [序号] 管理 (直接回车返回): ")
	var input string
	fmt.Scanln(&input)
	if input == "" { return }

	idx, _ := strconv.Atoi(input)
	if d, ok := dropletMap[idx]; ok {
		manageSingle(d.ID, d.Name, d.Region.Slug)
	}
}

func manageSingle(id int, name string, region string) {
	for {
		fmt.Printf("\n>>> 正在管理: %s <<<\n", name)
		fmt.Println("1. ⚡ 重启 / 关机 / 开机")
		fmt.Println("2. 🚀 无损换IP (保留数据/申请新保留IP)")
		fmt.Println("3. 🗑️ 彻底销毁 (自动清理绑定的保留IP)")
		fmt.Println("4. 返回上一级")
		fmt.Print("选择操作 (1-4): ")

		var op string
		fmt.Scanln(&op)
		switch op {
		case "1":
			fmt.Println("  [1]重启  [2]关机  [3]开机")
			var sub string
			fmt.Scanln(&sub)
			if sub == "1" { client.DropletActions.Reboot(ctx, id) }
			if sub == "2" { client.DropletActions.PowerOff(ctx, id) }
			if sub == "3" { client.DropletActions.PowerOn(ctx, id) }
			fmt.Println("✅ 电源指令已发送")
		case "2":
			changeReservedIP(id, region)
		case "3":
			deleteDropletAndIPs(id, name)
			return
		case "4":
			return
		}
	}
}

func changeReservedIP(id int, region string) {
	fmt.Println("⏳ 正在清理旧的保留IP并申请新IP...")
	ips, _, _ := client.ReservedIPs.List(ctx, nil)
	for _, ip := range ips {
		if ip.Droplet != nil && ip.Droplet.ID == id {
			client.ReservedIPs.Delete(ctx, ip.IP) 
		}
	}
	req := &godo.ReservedIPCreateRequest{Region: region, DropletID: id}
	newIP, _, err := client.ReservedIPs.Create(ctx, req)
	if err != nil {
		fmt.Printf("❌ 换IP失败: %v\n", err)
	} else {
		fmt.Printf("✅ 成功！新IP: %s (数据未丢失)\n", newIP.IP)
	}
}

func deleteDropletAndIPs(id int, name string) {
	fmt.Printf("⚠️ 确定销毁 %s 吗？关联的保留IP也会被清理防止扣费 (y/n): ", name)
	var conf string
	fmt.Scanln(&conf)
	if conf != "y" && conf != "Y" { return }

	ips, _, _ := client.ReservedIPs.List(ctx, nil)
	for _, ip := range ips {
		if ip.Droplet != nil && ip.Droplet.ID == id {
			client.ReservedIPs.Delete(ctx, ip.IP)
		}
	}
	client.Droplets.Delete(ctx, id)
	fmt.Println("✅ 机器及附属IP已全部销毁。")
}

// ==================== 2. 全定制创建模块 ====================

// 生成符合 DO 要求的强随机密码
func generateDOPassword() string {
	upper := "ABCDEFGHJKLMNPQRSTUVWXYZ"
	lower := "abcdefghijkmnopqrstuvwxyz"
	digits := "23456789"
	all := upper + lower + digits
	pwd := make([]byte, 12)
	pwd[0] = upper[rand.Intn(len(upper))]
	pwd[1] = digits[rand.Intn(len(digits))]
	for i := 2; i < 12; i++ {
		pwd[i] = all[rand.Intn(len(all))]
	}
	return string(pwd)
}

func customCreate() {
	// --- 第一步：选地区 ---
	fmt.Println("\n⏳ 正在获取全球地区...")
	regs, _, _ := client.Regions.List(ctx, &godo.ListOptions{PerPage: 100})
	var availRegs []godo.Region
	for _, r := range regs { if r.Available { availRegs = append(availRegs, r) } }
	
	fmt.Println("----------------------------------------")
	for i, r := range availRegs {
		name := r.Name
		if cn, ok := regionCN[r.Slug]; ok { name = cn }
		fmt.Printf("[%d] %-6s (%s)\n", i+1, r.Slug, name)
	}
	fmt.Println("----------------------------------------")
	fmt.Print("请选择地区序号: ")
	var rIdx int
	fmt.Scanln(&rIdx)
	if rIdx < 1 || rIdx > len(availRegs) { return }
	selReg := availRegs[rIdx-1]

	// --- 第二步：选系统镜像 ---
	fmt.Println("\n[系统镜像列表]")
	images := []struct{ Name, Slug string }{
		{"Ubuntu 24.04", "ubuntu-24-04-x64"},
		{"Ubuntu 22.04", "ubuntu-22-04-x64"},
		{"Debian 12", "debian-12-x64"},
		{"Debian 11", "debian-11-x64"},
		{"CentOS 9 Stream", "centos-stream-9-x64"},
	}
	for i, img := range images { fmt.Printf("[%d] %s\n", i+1, img.Name) }
	fmt.Print("请选择系统序号: ")
	var iIdx int
	fmt.Scanln(&iIdx)
	if iIdx < 1 || iIdx > len(images) { return }
	selImg := images[iIdx-1].Slug

	// --- 第三步：选 CPU / 硬盘类型 ---
	fmt.Println("\n[选择 CPU 与硬盘类型]")
	fmt.Println("[1] Regular (普通 CPU + 基础 SSD) - 最经济的方案，约 $4/月 起")
	fmt.Println("[2] Premium Intel (高级 Intel CPU + NVMe SSD) - 性能更强，约 $8/月 起")
	fmt.Println("[3] Premium AMD (高级 AMD CPU + NVMe SSD) - 性能更强，约 $8/月 起")
	fmt.Print("请选择硬件类型序号 (1-3): ")
	var typeIdx string
	fmt.Scanln(&typeIdx)

	// --- 第四步：根据硬盘类型过滤并选择配置 ---
	fmt.Println("\n⏳ 正在获取对应的价格配置...")
	sizes, _, _ := client.Sizes.List(ctx, &godo.ListOptions{PerPage: 200})
	var availSizes []godo.Size
	
	fmt.Println("--------------------------------------------------")
	count := 0
	for _, s := range sizes {
		// 只看基础款 (Basic), 且月付低于 $40
		if !strings.HasPrefix(s.Slug, "s-") || s.PriceMonthly > 40.0 || s.PriceMonthly <= 0 {
			continue
		}

		isIntel := strings.Contains(s.Slug, "-intel")
		isAMD := strings.Contains(s.Slug, "-amd")
		isRegular := !isIntel && !isAMD

		// 根据用户的选择进行过滤
		if typeIdx == "1" && !isRegular { continue }
		if typeIdx == "2" && !isIntel { continue }
		if typeIdx == "3" && !isAMD { continue }

		availSizes = append(availSizes, s)
		fmt.Printf("[%d] $%v/月 | %d核 CPU | %dMB 内存 | %dGB 硬盘\n", 
			count+1, s.PriceMonthly, s.Vcpus, s.Memory, s.Disk)
		count++
	}
	fmt.Println("--------------------------------------------------")
	if len(availSizes) == 0 {
		fmt.Println("❌ 该地区暂无所选类型的机器。")
		return
	}
	fmt.Print("请选择配置序号: ")
	var sIdx int
	fmt.Scanln(&sIdx)
	if sIdx < 1 || sIdx > len(availSizes) { return }
	selSize := availSizes[sIdx-1].Slug

	// --- 第五步：自定义初始脚本 (UserData) ---
	fmt.Println("\n[自定义初始脚本 UserData]")
	fmt.Println("[1] 不需要自定义 (仅使用系统默认优化配置)")
	fmt.Println("[2] 需要自定义 (自己粘贴多行 Shell 代码)")
	fmt.Print("请选择 (1-2): ")
	var scriptChoice string
	fmt.Scanln(&scriptChoice)

	var customScript string
	if scriptChoice == "2" {
		fmt.Println("\n👇 请在下方粘贴您的 Shell 代码 (输入大写 END 并回车结束):")
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			line := scanner.Text()
			if line == "END" { break }
			customScript += line + "\n"
		}
	} else {
		// 默认优化代码（装几个基础工具）
		customScript = "apt-get update && apt-get install -y curl wget htop\n"
	}

	// --- 第六步：自动生成与最终确认 ---
	autoName := fmt.Sprintf("%s-%d", selReg.Slug, rand.Intn(9000)+1000)
	rootPass := generateDOPassword()

	fmt.Println("\n================ [ 创建确认 ] ================")
	fmt.Printf("机器名称: %s\n", autoName)
	fmt.Printf("部署地区: %s\n", selReg.Slug)
	fmt.Printf("系统镜像: %s\n", selImg)
	fmt.Printf("硬件配置: %s\n", selSize)
	fmt.Printf("Root密码: %s  <-- (重要! 请截图或复制)\n", rootPass)
	fmt.Println("附加功能: 自动分配 IPv6 | 已注入 UserData 脚本")
	fmt.Println("==============================================")
	
	fmt.Print("\n确认开始创建？(y/n): ")
	var conf string
	fmt.Scanln(&conf)
	if conf != "y" && conf != "Y" { 
		fmt.Println("已取消。")
		return 
	}

	// 将系统必须的密码开通配置与用户的自定义脚本合并
	finalUserData := fmt.Sprintf(`#!/bin/bash
# 1. 配置 API 创建所需的 Root 密码
echo "root:%s" | chpasswd
sed -i 's/#PermitRootLogin.*/PermitRootLogin yes/g' /etc/ssh/sshd_config
sed -i 's/PasswordAuthentication no/PasswordAuthentication yes/g' /etc/ssh/sshd_config
systemctl restart sshd

# 2. 执行自定义脚本
%s
`, rootPass, customScript)

	req := &godo.DropletCreateRequest{
		Name:     autoName, 
		Region:   selReg.Slug, 
		Size:     selSize,
		Image:    godo.DropletCreateImage{Slug: selImg},
		IPv6:     true,
		UserData: finalUserData, // 注入合并后的脚本
	}

	fmt.Println("🚀 正在向 DigitalOcean 发送指令，请稍后...")
	_, _, err := client.Droplets.Create(ctx, req)
	if err != nil {
		fmt.Printf("❌ 创建失败: %v\n", err)
	} else {
		fmt.Println("\n✅ 机器部署成功！")
		fmt.Println("⚠️ 请务必保存好您的密码。")
		fmt.Println("提示: 机器启动需要约1-2分钟，您的代码正在后台运行。")
		fmt.Println("      请稍后在主菜单按 1 查看分配到的 IP。")
	}
}
