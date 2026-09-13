// 弹窗遮罩点击关闭指令:v-backdrop-close="handler"
// 仅当 mousedown 与 mouseup 都落在遮罩本身上时才触发关闭,
// 避免「在弹窗内拖选文本、鼠标移到遮罩上松开」时误关弹窗
// (此时 click 事件会被派发到按下点与松开点的公共祖先,恰好是遮罩层,
// 单纯的 @click.self 刚好命中)。
export default {
  mounted(el, binding) {
    let downOnBackdrop = false
    el._backdropMouseDown = (e) => {
      downOnBackdrop = e.target === el
    }
    el._backdropMouseUp = (e) => {
      if (downOnBackdrop && e.target === el && typeof binding.value === 'function') {
        binding.value()
      }
      downOnBackdrop = false
    }
    el.addEventListener('mousedown', el._backdropMouseDown)
    el.addEventListener('mouseup', el._backdropMouseUp)
  },
  unmounted(el) {
    el.removeEventListener('mousedown', el._backdropMouseDown)
    el.removeEventListener('mouseup', el._backdropMouseUp)
    delete el._backdropMouseDown
    delete el._backdropMouseUp
  },
}
