import DefaultTheme from 'vitepress/theme'
import { onMounted, watch, nextTick } from 'vue'
import { useRoute } from 'vitepress'
import type { Theme } from 'vitepress'

import './custom.css'

declare global {
  interface Window {
    mermaid: any
  }
}

let mermaidInitialized = false

const waitForMermaid = (): Promise<any> => {
  return new Promise((resolve, reject) => {
    const maxAttempts = 30
    let attempts = 0

    const check = () => {
      if (window.mermaid) {
        resolve(window.mermaid)
      } else if (attempts >= maxAttempts) {
        reject(new Error('Mermaid 加载超时'))
      } else {
        attempts++
        setTimeout(check, 200)
      }
    }

    check()
  })
}

const renderMermaid = async () => {
  await nextTick()
  const diagrams = document.querySelectorAll('.mermaid-diagram')
  if (diagrams.length === 0) return

  try {
    const mermaid = await waitForMermaid()

    // 初始化 mermaid（仅首次或主题切换时）
    if (!mermaidInitialized) {
      mermaid.initialize({
        startOnLoad: false,
        theme: document.documentElement.classList.contains('dark') ? 'dark' : 'default',
        securityLevel: 'loose',
        flowchart: {
          htmlLabels: true,
        },
      })
      mermaidInitialized = true
    }

    // 逐个渲染
    for (let i = 0; i < diagrams.length; i++) {
      const el = diagrams[i] as HTMLElement
      if (el.dataset.rendered === 'true') continue

      const id = `mermaid-${Date.now()}-${i}`
      // 从 data 属性读取并反转义 HTML 实体
      const rawContent = el.dataset.mermaid || el.textContent || ''
      const content = rawContent
        .replace(/&amp;/g, '&')
        .replace(/&quot;/g, '"')
        .replace(/&lt;/g, '<')
        .replace(/&gt;/g, '>')

      try {
        const { svg } = await mermaid.render(id, content)
        el.innerHTML = svg
        el.dataset.rendered = 'true'
      } catch (e) {
        console.error('Mermaid 渲染失败:', e)
        el.innerHTML = `<pre style="color:var(--vp-c-text-2)">Mermaid 渲染错误: ${e}</pre>`
      }
    }
  } catch (e) {
    console.error('Mermaid 加载失败:', e)
    diagrams.forEach(el => {
      el.innerHTML = `<pre style="color:var(--vp-c-text-2)">Mermaid 加载失败，请检查网络连接</pre>`
    })
  }
}

export default {
  extends: DefaultTheme,
  setup() {
    const route = useRoute()

    onMounted(() => {
      renderMermaid()

      // 监听深色模式切换
      const observer = new MutationObserver(() => {
        // 清除已渲染标记，重新渲染以适配主题
        document.querySelectorAll('.mermaid-diagram').forEach(el => {
          el.removeAttribute('data-rendered')
        })
        mermaidInitialized = false
        renderMermaid()
      })
      observer.observe(document.documentElement, {
        attributes: true,
        attributeFilter: ['class'],
      })
    })

    // 路由切换时重新渲染
    watch(
      () => route.path,
      () => {
        nextTick(() => renderMermaid())
      }
    )
  },
} satisfies Theme
