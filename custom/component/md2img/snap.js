const puppeteer = require('puppeteer-core');
const { marked } = require('marked');
const hljs = require('highlight.js');
const fs = require('fs');

const inputFile = process.argv[2];
const outputFile = process.argv[3];
const BROWSER_URL = process.env.CHROME_DEBUG_URL || 'http://127.0.0.1:9222';

if (!inputFile || !outputFile) {
    console.error('用法: node snap.js <源文件.md> <目的文件.png>');
    console.error('示例: node snap.js readme.md output.png');
    process.exit(1);
}

(async () => {
    let browser;
    try {
        if (!fs.existsSync(inputFile)) {
            console.error(`❌ 错误: 找不到文件 ${inputFile}`);
            process.exit(1);
        }
        const mdContent = fs.readFileSync(inputFile, 'utf-8');

        const htmlContent = marked.parse(mdContent, {
            highlight: (code, lang) => {
                const language = hljs.getLanguage(lang) ? lang : 'plaintext';
                return hljs.highlight(code, { language }).value;
            },
            langPrefix: 'hljs language-'
        });

        const fullHtml = `
        <!DOCTYPE html>
        <html>
        <head>
            <meta charset="UTF-8">
            <meta name="viewport" content="width=device-width, initial-scale=1.0">
            <link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/github-markdown-css/5.2.0/github-markdown.min.css">
            <link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/highlight.js/11.8.0/styles/tokyo-night-dark.min.css">
            <style>
                :root { 
                    --bg-gradient: linear-gradient(135deg, #f5f7fa 0%, #c3cfe2 100%); 
                }
                body { 
                    margin: 0; 
                    display: flex; 
                    justify-content: center; 
                    background: var(--bg-gradient); 
                    padding: 30px 10px;
                }
                .container {
                    background: white;
                    padding: 45px 35px;
                    border-radius: 16px;
                    box-shadow: 0 15px 35px rgba(0,0,0,0.1);
                    width: 750px; 
                    box-sizing: border-box;
                }
                .markdown-body {
                    font-family: "Noto Sans SC", "PingFang SC", "Microsoft YaHei", sans-serif !important;
                    font-size: 20px !important; 
                    line-height: 1.85 !important;
                    color: #2c3e50;
                    -webkit-font-smoothing: antialiased;
                }
                
                .markdown-body h1, .markdown-body h2, .markdown-body h3 {
                    font-weight: 600 !important;
                    color: #1a1a1a;
                    margin-top: 24px !important;
                    margin-bottom: 16px !important;
                }

                .markdown-body pre { 
                    border-radius: 12px !important; 
                    background-color: #1a1b26 !important;
                    padding: 24px !important;
                    font-size: 16px !important;
                    line-height: 1.6 !important;
                    overflow: hidden;
                    border: 1px solid #292e42;
                }

                .markdown-body pre code { 
                    font-family: 'Fira Code', 'Menlo', 'Monaco', monospace !important;
                    color: #cfc9c2 !important; 
                    background: transparent !important;
                    text-shadow: none !important;
                }

                .hljs {
                    display: block;
                    overflow-x: auto;
                    padding: 0;
                    color: #a9b1d6 !important; /* 默认浅蓝色文字 */
                    background: transparent !important;
                }
            </style>
        </head>
        <body>
            <div id="capture-target" class="container">
                <article class="markdown-body">
                    ${htmlContent}
                </article>
            </div>
        </body>
        </html>`;

        console.log(`🔌 正在连接浏览器: ${BROWSER_URL}`);
        try {
            browser = await puppeteer.connect({
                browserURL: BROWSER_URL,
                defaultViewport: null
            });
        } catch (err) {
            console.error(`❌ 无法连接浏览器，请确认已启动 Chrome 调试模式：`);
            console.error(`   命令示例: google-chrome --remote-debugging-port=9222 --user-data-dir=/tmp/chrome-profile-snap --headless=new --no-sandbox --disable-gpu --disable-dev-shm-usage`);
            console.error(`   或通过环境变量指定地址: CHROME_DEBUG_URL=http://127.0.0.1:9222 node snap.js ...`);
            process.exit(1);
        }

        const page = await browser.newPage();
        await page.setViewport({ width: 850, height: 1000, deviceScaleFactor: 1.5 });

        console.log('🎨 正在渲染内容...');
        await page.setContent(fullHtml, { waitUntil: 'networkidle0', timeout: 30000 });

        const element = await page.$('#capture-target');
        if (!element) {
            throw new Error('找不到 #capture-target 元素，请检查 HTML 结构');
        }

        await element.screenshot({
            path: outputFile,
            type: 'png',
            omitBackground: true
        });

        await page.close();
        console.log(`✨ 转换成功! 已保存为 ${outputFile}`);

    } catch (err) {
        console.error('💥 转换过程中出错:', err.message);
        process.exit(1);
    } finally{
        if (browser) {
        const timeout = setTimeout(() => {
            console.warn('⚠️ disconnect 超时，强制退出');
            process.exit(0);
        }, 2000);
        
        try {
            await browser.disconnect();
            clearTimeout(timeout);
            console.log('🔌 已断开浏览器连接');
        } catch (e) {
            clearTimeout(timeout);
            console.warn('⚠️ 断开连接时警告:', e.message);
        }
    }
    }
})();