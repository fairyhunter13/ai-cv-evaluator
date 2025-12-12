import sys
import os

def inject_key(key_path, config_path):
    print(f"Injecting key from {key_path} into {config_path}")
    indent = ' ' * 16
    
    try:
        with open(key_path, 'r') as f:
            # Prepend indentation to every line of the key
            key_content = ''.join([indent + line for line in f.readlines()])
        
        with open(config_path, 'r') as f:
            config_content = f.read()

        target = indent + '__OIDC_KEY_PLACEHOLDER__'
        if target in config_content:
            new_content = config_content.replace(target, key_content.rstrip())
            print("Placeholder found and replaced.")
        else:
            print("Indented placeholder not found, trying bare replacement.")
            new_content = config_content.replace('__OIDC_KEY_PLACEHOLDER__', key_content)

        with open(config_path, 'w') as f:
            f.write(new_content)
        print("Successfully injected key.")
            
    except Exception as e:
        print(f"Error: {e}")
        sys.exit(1)

if __name__ == "__main__":
    if len(sys.argv) != 3:
        print("Usage: inject_key.py <key_file> <config_file>")
        sys.exit(1)
    
    inject_key(sys.argv[1], sys.argv[2])
