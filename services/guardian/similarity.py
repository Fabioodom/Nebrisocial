import numpy as np

def evaluate_similarity(vec1: np.ndarray, vec2: np.ndarray) -> bool:
    """
    Calcula la similitud coseno entre dos vectores.
    Devuelve True si la similitud es estrictamente superior al umbral de 0.95 (umbral > 0.95),
    de lo contrario devuelve False.
    """
    norm_vec1 = np.linalg.norm(vec1)
    norm_vec2 = np.linalg.norm(vec2)
    
    if norm_vec1 == 0 or norm_vec2 == 0:
        return False
        
    cosine_sim = np.dot(vec1, vec2) / (norm_vec1 * norm_vec2)
    return float(cosine_sim) > 0.95
