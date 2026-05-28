import asyncio
from generator import CodeGenerator, build_llm
from config import settings

async def main():
    # 1. Construir el cliente LLM (Gemini/OpenAI)
    llm_client = build_llm(settings)
    
    # 2. Inyectarlo en el generador junto con las settings
    gen = CodeGenerator(llm=llm_client, settings=settings)
    
    print('Pidiendo diseño a la IA...')
    resultado = await gen.generate_component(
        'Una tarjeta (card) para mostrar la información de un volumen de manga, con espacio para la portada a la izquierda, título, autor, número de capítulos y un botón de Leer. Debe usar el Dark Mode de Nodal.',
        []
    )
    print('\n--- CÓDIGO GENERADO ---')
    print(resultado.templ_code)

if __name__ == "__main__":
    asyncio.run(main())