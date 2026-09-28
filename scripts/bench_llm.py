#!/usr/bin/env python3
"""Bench de modelos para el analizador de pedidos (Ollama).

Elige el modelo local mas chico que clasifique bien los pedidos reales.
Usa EXACTAMENTE el prompt de produccion (backend/internal/insight/prompts/
analyst.tmpl) y el catalogo real de la DB (tabla product_catalog), asi que lo
que se mide aca es lo que va a pasar en el servidor.

Uso:
    python3 scripts/bench_llm.py
    python3 scripts/bench_llm.py --models qwen2.5:3b-instruct-q4_K_M --runs 5
    python3 scripts/bench_llm.py --reps 3        # encolar en vez de chamar

Criterio de eleccion (el que se documento en AGENTS.md): gana el MAS CHICO con
>=90% de intents correctos y cero copias de los few-shot.
"""

import argparse
import json
import os
import pathlib
import statistics
import sys
import time
import urllib.error
import urllib.request

OLLAMA = os.environ.get("OLLAMA_BASE_URL", "http://localhost:11434")
PROMPT_PATH = (
    pathlib.Path(__file__).resolve().parents[1]
    / "backend/internal/insight/prompts/analyst.tmpl"
)

# Mismas tres banderas obligatorias que documenta AGENTS.md. Sin think:false el
# modelo de razonamiento alucina; sin el JSON Schema copia los few-shot.
SCHEMA = {
    "type": "object",
    "properties": {
        "intent": {"type": "string", "enum": ["pedido", "info", "reclamo", "otro"]},
        "resumen": {"type": "string"},
        "productos": {"type": "array", "items": {"type": "string"}},
        "cantidades": {"type": "array", "items": {"type": "integer"}},
        "material": {"type": ["string", "null"]},
        "medidas": {"type": ["string", "null"]},
        "personalizacion": {"type": ["string", "null"]},
        "fecha_entrega": {"type": ["string", "null"]},
        "confianza": {"type": "number"},
    },
    "required": ["intent", "resumen", "productos", "cantidades", "confianza"],
}

# Datos que SOLO existen en los few-shot del prompt. Si un resultado los trae,
# el modelo copio el ejemplo en vez de leer el mensaje.
EXAMPLE_DATA = ["3 macetas", "8 cm", "Cordoba", "devolucion del dinero",
               "horario de atencion"]

# (mensaje, intent esperado, que se revisa, es_conversacion, material, medidas)
CASES = [
    (
        "hola! buenas tardes, quiero hacerme un llavero en 3d para regalar. cuanto sale?",
        "pedido",
        "dice querer algo y preguntar precio: pedido con consulta de precio",
        False, None, None,
    ),
    (
        "Buen dia, quiero 4 macetas de 10cm en PETG, con el logo de mi estudio, para la semana que viene",
        "pedido",
        "producto, cantidad, material y medidas",
        False, "PETG", "10",
    ),
    (
        "me podrian pasar el precio del filamento PLA? para saber cuanto da el kilo",
        "info",
        "precio orientativo, sin intencion de comprar",
        False, "PLA", None,
    ),
    (
        "el pedido llego roto, tres de las cuatro piezas quedaron partidas. quiero que me lo rehagan",
        "reclamo",
        "reclamo con cantidad",
        False, None, None,
    ),
    ("ok gracias!", "otro", "no debe copiar el ejemplo de agradecimiento", False, None, None),
    (
        "Che, soy de Fulano, imprimen en PLA? hacen envios a Mendoza?",
        "info",
        "NO debe copiar 'Cordoba' del ejemplo",
        False, None, None,
    ),
    (
        "hola, saludos, que tal todo por alla?",
        "otro",
        "saludo puro, no debe copiar el ejemplo",
        False, None, None,
    ),
    (
        "Necesito un organizador de escritorio para la cables, en TPU negro, con 6 compartimentos, "
        "y lo quiero para el 15 de mayo",
        "pedido",
        "TPU + compartimentos + fecha de entrega",
        False, "TPU", None,
    ),
    # hilo: el pedido se acumula a lo largo de la conversacion
    (
        """Cliente: hola, quiero una piececita
Agente: Hola! Que necesitarias?
Cliente: un portaobjetos para el celu
Agente: Claro. De que material lo queres?
Cliente: en PETG, color negro. Y queria ver si hacen envio a Rosario
Agente: Si hacemos envios. Lo queres impreso en 3D?
Cliente: si, en 3D, uno nomas""",
        "pedido",
        "consolida el hilo: producto + material + envio",
        True, "PETG", None,
    ),
    (
        """Cliente: Buen dia
Agente: Hola, en que te puedo ayudar?
Cliente: I was wondering if you could quote 2000 business cards. Do you do that?
Agente: Si, hacemos tarjetas. De que medida y papel?
Cliente: 9x5, papel 300g, con el logo adelante""",
        "pedido",
        "hilo con typos y code-switching (imprenta, fuera del catalogo 3D)",
        True, "300", "9x5",
    ),
]


def fetch_catalog() -> str:
    """Arma el bloque CATALOGO igual que lo hace el backend."""
    try:
        import subprocess

        sql = (
            "SELECT category, name, aliases FROM product_catalog "
            "WHERE active ORDER BY category, sort_order"
        )
        out = subprocess.run(
            ["docker", "exec", "crm-postgres", "psql", "-U", "crm", "-d", "crm",
             "-t", "-A", "-F", "|", "-c", sql],
            capture_output=True, text=True, timeout=15, check=True,
        ).stdout.strip()
    except Exception as exc:  # sin DB: el bench corre igual, sin catalogo
        print(f"! no pude leer el catalogo ({exc}); sigo sin bloque CATALOGO",
              file=sys.stderr)
        return ""

    by_cat: dict[str, list[str]] = {}
    for line in out.splitlines():
        if not line.strip():
            continue
        cat, name, aliases = (line.split("|") + ["", ""])[:3]
        label = name
        alias_list = [
            a for a in aliases.strip("{}").split(",") if a.strip()
        ]
        if alias_list:
            label += " (" + ", ".join(sorted(a.strip() for a in alias_list[:3])) + ")"
        by_cat.setdefault(cat, []).append(label)

    if not by_cat:
        return ""

    lines = [
        "## CATALOGO DE PRODUCTOS",
        'Usá estos nombres exactos en "productos" cuando el cliente mencione o',
        "describa algo que exista aca. Los parentesis son palabras que el cliente",
        "usa para referirse a ese producto:",
    ]
    for cat, items in by_cat.items():
        lines.append(f"- {cat}: " + ", ".join(items))
    lines.append("")
    return "\n".join(lines)


def build_prompt(catalog: str) -> str:
    return PROMPT_PATH.read_text(encoding="utf-8").replace("{{catalog}}", catalog)


def post(path: str, body: dict, timeout: int = 300) -> dict:
    req = urllib.request.Request(
        OLLAMA + path, json.dumps(body).encode(), {"Content-Type": "application/json"}
    )
    with urllib.request.urlopen(req, timeout=timeout) as resp:
        return json.load(resp)


def chat(model: str, prompt: str, text: str) -> dict:
    t0 = time.time()
    out = post(
        "/api/chat",
        {
            "model": model,
            "stream": False,
            "think": False,
            "format": SCHEMA,
            "keep_alive": "30m",
            "options": {"temperature": 0, "num_ctx": 8192, "num_predict": 400},
            "messages": [
                {"role": "system", "content": prompt},
                {"role": "user", "content": "Mensajes del cliente:\n" + text},
            ],
        },
    )
    return {
        "wall_s": time.time() - t0,
        "prompt_tok": out.get("prompt_eval_count", 0),
        "out_tok": out.get("eval_count", 0),
        "content": out["message"].get("content", ""),
    }


def score(case, parsed: dict) -> list[str]:
    """Devuelve la lista de fallas (vacia = el caso paso bien)."""
    text, expected = case[0], case[1]
    want_material, want_medidas = case[4], case[5]
    fails = []

    if expected != parsed.get("intent"):
        fails.append(f"intent {parsed.get('intent')!r} != {expected!r}")

    blob = json.dumps(parsed, ensure_ascii=False).lower()
    for token in EXAMPLE_DATA:
        low = token.lower()
        if low in blob and low not in text.lower():
            fails.append(f"copio el ejemplo: {token!r}")

    resumen = (parsed.get("resumen") or "").strip()
    if not resumen:
        fails.append("resumen vacio")
    elif len(resumen) > 160:
        fails.append(f"resumen desmedido ({len(resumen)} car)")

    # calidad del detalle: si el cliente lo dijo, tiene que haber salido
    material = (parsed.get("material") or "").strip()
    if want_material and want_material.lower() not in material.lower():
        fails.append(f"material {material!r} no contiene {want_material!r}")
    medidas = (parsed.get("medidas") or "").strip()
    if want_medidas and not medidas:
        fails.append("medidas vacias (el cliente dio medidas)")
    elif want_medidas and want_medidas in ("10", "9x5") and want_medidas not in medidas:
        fails.append(f"medidas {medidas!r} no contiene {want_medidas!r}")

    for key in ("productos", "cantidades"):
        if not isinstance(parsed.get(key), list):
            fails.append(f"{key} no es lista")
    return fails



def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--models", nargs="*", default=None)
    ap.add_argument("--runs", type=int, default=1, help="repeticiones por caso (para p50)")
    ap.add_argument("--reps", type=int, default=1, help="veces que se repite cada caso")
    args = ap.parse_args()

    catalog = fetch_catalog()
    prompt = build_prompt(catalog)
    print(f"prompt: {len(prompt)} chars, catalogo: {len(catalog)} chars, "
          f"casos: {len(CASES)}, reps: {args.reps}\n")

    models = args.models
    if not models:
        tags = json.load(urllib.request.urlopen(OLLAMA + "/api/tags", timeout=10))
        have = {m["name"] for m in tags["models"]}
        models = [m for m in ("granite3.3:2b", "qwen3:1.7b",
                              "qwen2.5:3b-instruct-q4_K_M", "lfm2.5:latest")
                  if m in have]

    results = {}
    for model in models:
        sizes = {m["name"]: m["size"] for m in
                 json.load(urllib.request.urlopen(OLLAMA + "/api/tags", timeout=10))["models"]}
        print(f"\n{'='*78}\n{model}  ({sizes.get(model,0)/1e9:.2f} GB)\n{'='*78}")

        latencies, failures, json_errors = [], [], 0
        for case in CASES:
            text, expected = case[0], case[1]
            for _ in range(args.reps):
                try:
                    res = chat(model, prompt, text)
                except (urllib.error.URLError, TimeoutError) as exc:
                    json_errors += 1
                    print(f"  ERROR de red: {exc}")
                    continue
                latencies.append(res["wall_s"])
                try:
                    parsed = json.loads(res["content"])
                except json.JSONDecodeError:
                    json_errors += 1
                    print(f"  JSON INVALIDO  [{expected}] {res['content'][:110]!r}")
                    continue
                fails = score(case, parsed)
                mark = "ok  " if not fails else "FAIL"
                if fails:
                    failures.append((text, fails))
                intent = parsed.get("intent")
                detail = f" mat={parsed.get('material') or '-'} med={parsed.get('medidas') or '-'}"
                print(f"  {mark} [{str(intent):<6}] {res['wall_s']:.2f}s "
                      f"{str(parsed.get('resumen'))[:52]!r}{detail if fails else ''}")
                if fails:
                    print(f"        -> {', '.join(fails)}")

        total = len(CASES) * args.reps
        ok = total - len(failures) - json_errors
        results[model] = {
            "size_gb": round(sizes.get(model, 0) / 1e9, 2),
            "passed": ok, "total": total,
            "pct": round(100 * ok / total, 1),
            "p50": round(statistics.median(latencies), 2) if latencies else None,
            "max": round(max(latencies), 2) if latencies else None,
        }


    print(f"\n{'='*78}\nRESUMEN (elegir el MAS CHICO con >=90% y cero copias)\n{'='*78}")
    print(f"{'model':<32}{'RAM':>7}{'ok':>10}{'p50':>8}{'max':>8}")
    for model, r in sorted(results.items(), key=lambda kv: kv[1]["size_gb"]):
        print(f"{model:<32}{r['size_gb']:>6.2f}G"
              f"{r['passed']:>5}/{r['total']:<4}{r['p50']:>7.2f}s{r['max']:>7.2f}s"
              f"   {r['pct']}%")
    return 0


if __name__ == "__main__":
    sys.exit(main())
