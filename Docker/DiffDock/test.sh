#!/bin/bash

source activate diffdock &&
python datasets/esm_embedding_preparation.py --protein_path /inputs/6d08/6d08_protein_processed.pdb --out_file /outputs/prepared_for_esm.fasta &&
HOME=esm/model_weights python esm/scripts/extract.py esm2_t33_650M_UR50D /outputs/prepared_for_esm.fasta /outputs/esm2_output --repr_layers 33 --include per_tok &&
python -m inference --protein_path /inputs/6d08/6d08_protein_processed.pdb --ligand /inputs/6d08/6d08_ligand.sdf --out_dir /outputs --inference_steps 20 --samples_per_complex 40 --batch_size 10 --actual_steps 18 --esm_embeddings_path /outputs/esm2_output --no_final_step_noise
