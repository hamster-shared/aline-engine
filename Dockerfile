FROM aptoslabs/tools:aptos-node-v1.2.7
RUN apt update && apt install -y git

RUN git clone https://github.com/aptos-labs/aptos-core.git