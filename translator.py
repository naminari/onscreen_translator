import sys
import io
from transformers import MarianTokenizer, MarianMTModel

sys.stdin = io.TextIOWrapper(sys.stdin.buffer, encoding='utf-8')
sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8')

model_name = "Helsinki-NLP/opus-mt-en-ru"
print("[NMT] Loading model...", file=sys.stderr)
tokenizer = MarianTokenizer.from_pretrained(model_name)
model = MarianMTModel.from_pretrained(model_name)
print("[NMT] Ready", file=sys.stderr)
sys.stderr.flush()

for line in sys.stdin:
    text = line.strip()
    if not text:
        continue
    inputs = tokenizer(text, return_tensors="pt", truncation=True, max_length=512)
    outputs = model.generate(**inputs, max_length=1024)
    translation = tokenizer.decode(outputs[0], skip_special_tokens=True)
    print(translation)
    sys.stdout.flush()