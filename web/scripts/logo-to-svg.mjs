// 一次性工具：把 ASCII LOGO 转成 SVG path 数据（游程合并），用于生成 components/AsciiLogo.vue
// 用法: node scripts/logo-to-svg.mjs

// 原始字符画（ANSI Shadow 字体的 "AniaBot"，█ = 实心块，░ = 轻影块）
const LOGO = `
   █████████               ███            ███████████            █████   
  ███░░░░░███             ░░░            ░░███░░░░░███          ░░███    
 ░███    ░███  ████████   ████   ██████   ░███    ░███  ██████  ███████  
 ░███████████ ░░███░░███ ░░███  ░░░░░███  ░██████████  ███░░███░░░███░   
 ░███░░░░░███  ░███ ░███  ░███   ███████  ░███░░░░░███░███ ░███  ░███    
 ░███    ░███  ░███ ░███  ░███  ███░░███  ░███    ░███░███ ░███  ░███ ███
 █████   █████ ████ █████ █████░░████████ ███████████ ░░██████   ░░█████ 
░░░░░   ░░░░░ ░░░░ ░░░░░ ░░░░░  ░░░░░░░░ ░░░░░░░░░░░   ░░░░░░     ░░░░░  
`
const lines = LOGO.replace(/^\n/, '').replace(/\n$/, '').split('\n')

const CELL_W = 1
const CELL_H = 1.8 // 模拟等宽字体行距（字宽:行高 ≈ 1:1.8），保持原有比例
const GX = 0.07 // 水平留缝，模拟终端块状像素观感
const GY = 0.14 // 垂直留缝

const r2 = (n) => Number(n.toFixed(2))

function pathsFor(ch) {
  const parts = []
  lines.forEach((line, row) => {
    let col = 0
    while (col < line.length) {
      if (line[col] !== ch) { col++; continue }
      let end = col
      while (end < line.length && line[end] === ch) end++
      const x = r2(col * CELL_W + GX)
      const y = r2(row * CELL_H + GY)
      const w = r2((end - col) * CELL_W - GX * 2)
      const h = r2(CELL_H - GY * 2)
      parts.push(`M${x} ${y}h${w}v${h}h-${w}z`)
      col = end
    }
  })
  return parts.join('')
}

console.log('W:', Math.max(...lines.map(l => l.length)), 'H:', lines.length * CELL_H)
console.log('SOLID:', pathsFor('█'))
console.log('DIM:', pathsFor('░'))
