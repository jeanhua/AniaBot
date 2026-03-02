pandoc source.md -o output.html --standalone --embed-resources && \
wkhtmltoimage --width 750 --quality 100 output.html output.jpg && \
rm -f output.html