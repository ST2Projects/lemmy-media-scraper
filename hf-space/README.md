---
title: Image Recognition API
emoji: 👁️
colorFrom: blue
colorTo: purple
sdk: gradio
sdk_version: 5.9.1
app_file: app.py
pinned: false
license: apache-2.0
suggested_hardware: a10g-small
---

# Image Recognition API

A vision model API for image analysis and tagging using Qwen2.5-VL-7B-Instruct.

## Features

- **Image Description**: Get detailed descriptions of any image
- **Tag Generation**: Automatically generate classification tags in JSON format
- **Custom Prompts**: Use your own prompts for specific analysis needs
- **API Access**: Full Gradio API for programmatic access

## API Usage

### Python Client

```python
from gradio_client import Client

client = Client("neo1908/image-recognition-api")

# Describe an image
result = client.predict(
    image="path/to/image.jpg",
    prompt="Describe this image in detail",
    max_tokens=512,
    api_name="/analyze_image"
)
print(result)

# Generate tags
tags = client.predict(
    image="path/to/image.jpg",
    num_tags=10,
    api_name="/generate_tags"
)
print(tags)
```

### HTTP API

```bash
curl -X POST https://neo1908-image-recognition-api.hf.space/api/predict \
  -H "Content-Type: application/json" \
  -d '{"data": ["<base64_image>", "Describe this image", 512]}'
```

## Model

This Space uses [Qwen2.5-VL-7B-Instruct](https://huggingface.co/Qwen/Qwen2.5-VL-7B-Instruct), a state-of-the-art vision-language model from Alibaba.

## License

Apache 2.0
