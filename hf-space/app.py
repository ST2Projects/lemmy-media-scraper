import gradio as gr
from transformers import Qwen2_5_VLForConditionalGeneration, AutoProcessor
from qwen_vl_utils import process_vision_info
import torch
from PIL import Image
import json

# Verify CUDA availability
print(f"PyTorch version: {torch.__version__}")
print(f"CUDA available: {torch.cuda.is_available()}")
if torch.cuda.is_available():
    print(f"CUDA device: {torch.cuda.get_device_name(torch.cuda.current_device())}")
    print(f"CUDA version: {torch.version.cuda}")
else:
    print("WARNING: CUDA not available, running on CPU (this will be slow)")

# Determine device
device = "cuda" if torch.cuda.is_available() else "cpu"

# Load model on startup
model_id = "Qwen/Qwen2.5-VL-7B-Instruct"

print(f"Loading model {model_id}...")
print(f"Using device: {device}")

if torch.cuda.is_available():
    model = Qwen2_5_VLForConditionalGeneration.from_pretrained(
        model_id,
        torch_dtype=torch.float16,
        device_map="auto",
        trust_remote_code=True
    )
else:
    # CPU fallback (will be slow)
    model = Qwen2_5_VLForConditionalGeneration.from_pretrained(
        model_id,
        torch_dtype=torch.float32,
        trust_remote_code=True
    )
    model = model.to(device)

processor = AutoProcessor.from_pretrained(model_id, trust_remote_code=True)
print("Model loaded successfully!")
print(f"Model device: {next(model.parameters()).device}")


def analyze_image(image, prompt, max_tokens):
    """Analyze an image and return a detailed description."""
    if image is None:
        return "Please upload an image."

    messages = [
        {
            "role": "user",
            "content": [
                {"type": "image", "image": image},
                {"type": "text", "text": prompt}
            ]
        }
    ]

    text = processor.apply_chat_template(messages, tokenize=False, add_generation_prompt=True)
    image_inputs, video_inputs = process_vision_info(messages)
    inputs = processor(
        text=[text],
        images=image_inputs,
        videos=video_inputs,
        padding=True,
        return_tensors="pt"
    ).to(model.device)

    with torch.no_grad():
        output_ids = model.generate(
            **inputs,
            max_new_tokens=max_tokens,
            do_sample=True,
            temperature=0.7,
            top_p=0.9
        )

    output_text = processor.batch_decode(
        output_ids[:, inputs.input_ids.shape[1]:],
        skip_special_tokens=True
    )[0]

    return output_text


def generate_tags(image, num_tags):
    """Generate classification tags for an image."""
    if image is None:
        return "Please upload an image."

    tag_prompt = f"""Analyze this image and provide exactly {num_tags} descriptive tags.
Output ONLY a JSON array of strings, no other text.
Tags should describe: objects, scene type, colors, mood, style, actions, and subjects.
Example format: ["tag1", "tag2", "tag3"]"""

    messages = [
        {
            "role": "user",
            "content": [
                {"type": "image", "image": image},
                {"type": "text", "text": tag_prompt}
            ]
        }
    ]

    text = processor.apply_chat_template(messages, tokenize=False, add_generation_prompt=True)
    image_inputs, video_inputs = process_vision_info(messages)
    inputs = processor(
        text=[text],
        images=image_inputs,
        videos=video_inputs,
        padding=True,
        return_tensors="pt"
    ).to(model.device)

    with torch.no_grad():
        output_ids = model.generate(
            **inputs,
            max_new_tokens=256,
            do_sample=False
        )

    output_text = processor.batch_decode(
        output_ids[:, inputs.input_ids.shape[1]:],
        skip_special_tokens=True
    )[0]

    # Try to parse as JSON, return raw if parsing fails
    try:
        tags = json.loads(output_text.strip())
        if isinstance(tags, list):
            return json.dumps(tags, indent=2)
    except json.JSONDecodeError:
        pass

    return output_text


# Gradio interface with tabs
with gr.Blocks(title="Image Recognition API") as demo:
    gr.Markdown("# Image Recognition API")
    gr.Markdown("Upload an image for detailed analysis or tag generation using Qwen2.5-VL-7B.")

    with gr.Tabs():
        with gr.TabItem("Describe Image"):
            with gr.Row():
                with gr.Column():
                    img_input = gr.Image(type="pil", label="Upload Image")
                    prompt_input = gr.Textbox(
                        label="Custom Prompt",
                        placeholder="Describe this image...",
                        value="Describe this image in detail, including all objects, people, activities, text, and any notable features."
                    )
                    max_tokens = gr.Slider(
                        minimum=64,
                        maximum=1024,
                        value=512,
                        step=64,
                        label="Max Tokens"
                    )
                    describe_btn = gr.Button("Analyze", variant="primary")
                with gr.Column():
                    description_output = gr.Textbox(label="Description", lines=10)

            describe_btn.click(
                fn=analyze_image,
                inputs=[img_input, prompt_input, max_tokens],
                outputs=description_output,
                api_name="analyze_image"
            )

        with gr.TabItem("Generate Tags"):
            with gr.Row():
                with gr.Column():
                    tag_img_input = gr.Image(type="pil", label="Upload Image")
                    num_tags = gr.Slider(
                        minimum=5,
                        maximum=20,
                        value=10,
                        step=1,
                        label="Number of Tags"
                    )
                    tag_btn = gr.Button("Generate Tags", variant="primary")
                with gr.Column():
                    tags_output = gr.Textbox(label="Tags (JSON)", lines=10)

            tag_btn.click(
                fn=generate_tags,
                inputs=[tag_img_input, num_tags],
                outputs=tags_output,
                api_name="generate_tags"
            )

    gr.Markdown("---")
    gr.Markdown("**Model:** Qwen2.5-VL-7B-Instruct | **License:** Apache 2.0")

# Launch configured for HuggingFace Spaces
# show_api=False prevents gradio from generating API docs that crash with Qwen model types
demo.launch(server_name="0.0.0.0", server_port=7860, show_api=False)
