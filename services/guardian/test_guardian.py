import numpy as np
from similarity import evaluate_similarity

def test_exact_duplicate():
    vec1 = np.array([1.0, 2.0, 3.0, 4.0])
    vec2 = np.array([1.0, 2.0, 3.0, 4.0])
    # Identical vectors should have a cosine similarity of 1.0, which is > 0.95, so True
    assert evaluate_similarity(vec1, vec2) is True

def test_different_topics():
    vec1 = np.array([1.0, 0.0, 0.0])
    vec2 = np.array([0.0, 1.0, 0.0])
    # Orthogonal vectors should have a cosine similarity of 0.0, which is <= 0.95, so False
    assert evaluate_similarity(vec1, vec2) is False

def test_edge_case_threshold():
    u = np.array([1.0, 0.0])
    
    # Cosine similarity of exactly 0.94
    v_94 = np.array([0.94, np.sqrt(1 - 0.94**2)])
    # 0.94 <= 0.95, so it should return False
    assert evaluate_similarity(u, v_94) is False
    
    # Cosine similarity of exactly 0.96
    v_96 = np.array([0.96, np.sqrt(1 - 0.96**2)])
    # 0.96 > 0.95, so it should return True
    assert evaluate_similarity(u, v_96) is True
