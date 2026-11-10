from fastapi import FastAPI

app = FastAPI(title="ante-scorer")


@app.get("/healthz")
def healthz() -> dict[str, str | None]:
    return {"status": "ok", "model_version": None}
