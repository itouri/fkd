package main

import (
  "bytes"
  "fmt"
  "os"
  "unsafe"
  "encoding/binary"
)

const (
  IALLC = 0100000
  IFMT = 060000
  IFDIR = 040000
  IFCHR = 020000
  IFBLK = 060000
  ILARG = 010000
  ISUID = 04000
  ISGID = 02000
  ISVTX = 01000
  IREAD = 0400
  IWRITE = 0200
  IEXEC = 0100

  BLOCK_BYTES = 512
  ROOT_INODE = 0
)

type Filsys struct {
  S_isize int16
  S_fsize int16
  S_nfree int16
  S_free[100] int16
  S_ninod int16
  S_inode[100] int16
  S_flock int8
  S_ilock int8
  S_fmod int8
  S_ronly int8
  S_time[2] int16
  Pad[48] int16
}

type Ino struct {
  I_mode uint16
  I_nlink int8
  I_uid int8
  I_gid int8
  I_size0 uint8
  I_size1 uint16
  I_addr[8] uint16
  I_atime[2] int16
  I_mtime[2] int16
}

type Directory struct {
  Inode uint16
  Name[14] byte
}

// グローバル化するのはどうかな
var fs_inodes []Ino
var storage [][]uint8
var inode_block_size int16
var current_dir uint16

// カレントディレクトリのinodeのブロックのファイルリストをパース
func parseDirName() []Directory {
  dir_names := []Directory{}

  // 2 は ブート領域 ＋ スーパーブロック
  var offset uint16 = uint16(inode_block_size) + 2

  for _, addr := range fs_inodes[current_dir].I_addr {
    if addr == 0 {
      break
    }

    // C -> memcpy(dn, sizeof(Directory), storage[addr - offset])
    buf := &bytes.Buffer{}
    buf.Write(storage[addr - offset])
    // Directoryをパース
    for true {
      var dn Directory
      binary.Read(buf, binary.LittleEndian, &dn)
      if dn.Inode == 0 {
        break
      }
      dir_names = append(dir_names, dn)
    }
  }
  return dir_names
}

func ls(opt bool)  {
  dir_names := parseDirName()
  for _, a := range dir_names {
    if opt {
      pre := ""
      inode := fs_inodes[a.Inode - 1]
      file_size := int(inode.I_size0) * 0x10000 + int(inode.I_size1)

      if inode.I_mode & IFDIR != 0 {
        pre += "d"
      } else {
        pre += "-"
      }

      // rwxrwxrwx の部分
      var i uint
      for i = 0; i < 3; i++ {
        mode := inode.I_mode << (i * 3)
        if mode & IREAD != 0 {
          pre += "r"
        } else {
          pre += "-"
        }
        if mode & IWRITE != 0 {
          pre += "w"
        } else {
          pre += "-"
        }
        if mode & IEXEC != 0 {
          pre += "x"
        } else {
          pre += "-"
        }
      }
      fmt.Printf("%s %8d %s\n", pre, file_size, a.Name)
    } else {
      fmt.Printf("%s\n", a.Name)
    }
  }
}

func cd(path string)  {
  dir_names := parseDirName()
  for _, d := range dir_names {
    // d.Nameから 00 の部分を削除
    var dir_name_str []byte
    for i, dn := range d.Name {
      if dn == 0 {
        dir_name_str = d.Name[0:i]
        break // これを忘れてたせいで,だいぶ悩んだ
      }
    }

    // rootへ移動
    if path == "/" {
      current_dir = ROOT_INODE
      return
    }

    if path == string(dir_name_str) {
      // cd先のディレクトリフラグが立っている
      if (fs_inodes[d.Inode - 1].I_mode & IFDIR) != 0 {
        current_dir = d.Inode - 1
        return
      } else {
        fmt.Printf("%s is not directory\n", path)
        return
      }
    }
  }
  fmt.Printf("can not found %s\n", path)
}

func main()  {
  file_name := "./v6root"

  file, err := os.Open(file_name)
  if err != nil {
    fmt.Printf("cant read  %s", file_name)
    return
  }

  var super_block Filsys
  buf := make([]byte, unsafe.Sizeof(super_block)) // 512B
  //  起動用ブロックは読み捨て
  file.Read(buf)
  // スーパーブロック
  file.Read(buf)
  binary.Read(bytes.NewBuffer(buf), binary.LittleEndian, &super_block)

  // ただ名前変えてるだけ
  inode_block_size = super_block.S_isize // 最初の初期化
  var file_block_size int16 = super_block.S_fsize

  // 16 = 512 / 32 1ブロックあたりのinode数
  // C -> FSInode inode_block_size[inode_block_size]
  fs_inodes = make([]Ino, inode_block_size * 16)
  buf = make([]byte, unsafe.Sizeof(fs_inodes[0])) // 32B
  for i, _  := range fs_inodes {
    file.Read(buf)
    binary.Read(bytes.NewBuffer(buf), binary.LittleEndian, &fs_inodes[i])
  }

  buf = make([]byte, BLOCK_BYTES)
  // unsigned char storage[file_block_size][512]
  storage = make([][]uint8, file_block_size)
  for i, _ := range storage {
    storage[i] = make([]uint8, BLOCK_BYTES)
    file.Read(buf)
    binary.Read(bytes.NewBuffer(buf), binary.LittleEndian, &storage[i])
  }

  // ルードディレクトリのファイルリスト取り出す
  current_dir = ROOT_INODE

  var cmd string
  var opt string
  for ;; {
    fmt.Printf("$> ")
    opt = ""
    fmt.Scanf("%s %s", &cmd, &opt)

    switch cmd {
    case "ls":
      if opt == "-l" {
        ls(true)
      } else {
        ls(false)
      }
    case "cd":
      if opt == "" {
        fmt.Printf("type dir name\n")
        continue
      }
      cd(opt)
    default:
      fmt.Printf("%s is unkwon\n", cmd)
    }
  }
}
