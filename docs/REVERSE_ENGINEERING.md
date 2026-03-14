# Reverse Engineering du protocole IPC MetaTrader 5

## Objectif

Comprendre le protocole binaire entre `MetaTrader5.pyd` et `terminal64.exe`
pour creer un binding Go natif equivalent.

## Prerequis

- Windows 10/11 avec MT5 installe et un compte demo actif
- Python 3.10+ avec `pip install MetaTrader5 pefile capstone`
- Ghidra (https://ghidra-sre.org) pour le desassemblage
- Sysinternals Process Monitor (https://learn.microsoft.com/en-us/sysinternals/downloads/procmon)

## Procedure

### Etape 1 : Enumerer les pipes (5 min)

```powershell
# Lancer MT5, ouvrir un chart, attendre la connexion au serveur

# Lister les pipes MT5
python tools/sniffer/pipe_sniffer.py --enum
```

Resultat attendu : une liste de pipes contenant "MT5", "Meta", ou "Terminal".
Noter le nom exact du pipe.

### Etape 2 : Analyser le .pyd statiquement (10 min)

```powershell
python tools/analyzer/extract_pyd_info.py
```

Ce script extrait :
- Les imports DLL (kernel32: CreateFileW, ReadFile, WriteFile...)
- Les exports (PyInit_MetaTrader5)
- Les strings (noms de pipe, messages d'erreur, noms de fonctions)
- Les sites d'appel a CreateFileW/ReadFile/WriteFile (si capstone installe)

**Objectif :** trouver le nom du pipe dans les strings du binaire.

### Etape 3 : Capturer le trafic avec Process Monitor (15 min)

1. Lancer Process Monitor en administrateur
2. Ajouter les filtres :
   ```
   Process Name  is  python.exe     Include
   Process Name  is  terminal64.exe Include
   Path          contains  pipe     Include
   ```
3. Effacer la capture (Ctrl+X)
4. Dans un autre terminal :
   ```powershell
   python tools/sniffer/pipe_sniffer.py
   ```
5. Arreter la capture
6. File > Save as CSV
7. Analyser :
   ```powershell
   python tools/sniffer/procmon_capture.py --analyze export.csv
   ```

**Objectif :** voir le nom exact du pipe, les tailles des messages, l'ordre des operations.

### Etape 4 : Sniffer le trafic en temps reel (20 min)

```powershell
python tools/sniffer/dll_hook_sniffer.py
```

Ce script :
- Trouve le processus terminal64.exe
- Enumere les pipes
- Execute chaque fonction MT5 Python
- Tente de capturer le trafic

### Etape 5 : MITM Pipe Proxy (30 min)

C'est l'approche la plus fiable. On cree un proxy entre Python et MT5.

```powershell
# 1. Trouver le vrai pipe
python tools/sniffer/mitm_pipe.py --standalone

# 2. Supposons que le pipe est \\.\pipe\MT5.Terminal.XYZ
#    On cree un proxy :
#    - Notre faux pipe : \\.\pipe\MT5.Terminal.XYZ  (on le prend)
#    - Le vrai pipe : on le renomme ou on attend qu'il soit recree

# 3. Methode alternative : utiliser un chemin custom pour initialize()
#    Modifier le path dans le script Python pour pointer vers notre faux pipe

python tools/sniffer/mitm_pipe.py --fake-pipe "\\.\pipe\MT5.Proxy" --real-pipe "\\.\pipe\MT5.Terminal.XYZ"
```

### Etape 6 : Ghidra - Desassemblage du .pyd (1-2h)

Suivre le guide : `tools/analyzer/ghidra_guide.md`

Points cles a analyser :
1. **PyInit_MetaTrader5** : table PyMethodDef = mapping nom -> fonction C
2. **Fonction initialize** : sequence de connexion au pipe
3. **Fonctions qui appellent WriteFile** : format des requetes
4. **Fonctions qui appellent ReadFile** : format des reponses
5. **Structures de donnees** : layout des messages binaires

### Etape 7 : Reconstruction du protocole

Avec les donnees collectees, reconstruire :

```
Message Request:
    [4 bytes] total_size (LE? BE?)
    [4 bytes] command_id
    [4 bytes] request_id / sequence
    [N bytes] parameters (serialisation a determiner)

Message Response:
    [4 bytes] total_size
    [4 bytes] command_id (echo?)
    [4 bytes] request_id
    [4 bytes] status_code
    [N bytes] response_data
```

### Etape 8 : Prototype Go

Une fois le protocole compris, le binding Go dans `pkg/mt5/` :

```go
// internal/pipe/pipe_windows.go
package pipe

import (
    "golang.org/x/sys/windows"
)

func Connect(pipeName string) (*Conn, error) {
    handle, err := windows.CreateFile(
        windows.StringToUTF16Ptr(pipeName),
        windows.GENERIC_READ|windows.GENERIC_WRITE,
        0, nil,
        windows.OPEN_EXISTING,
        0, 0,
    )
    // ...
}
```

## Donnees a collecter

Pour chaque fonction API, noter :

| Fonction | Cmd ID | Request size | Request format | Response size | Response format |
|----------|--------|-------------|----------------|--------------|-----------------|
| initialize | ? | ? | ? | ? | ? |
| version | ? | ? | ? | ? | ? |
| account_info | ? | ? | ? | ? | ? |
| ... | | | | | |

## Risques

- MetaQuotes peut changer le protocole a chaque mise a jour
- Le pipe name peut varier selon l'installation
- Le binaire peut avoir des protections anti-debug
- Le protocole peut inclure un challenge/response

## Fichiers de sortie

```
mt5_pipe_trace/          # Traces du pipe_sniffer
mt5_pipe_deep_trace/     # Traces du dll_hook_sniffer
mt5_mitm_trace/          # Traces du MITM proxy
mt5_pyd_analysis/        # Analyse statique du .pyd
mt5_procmon_analysis/    # Analyse Process Monitor
```
