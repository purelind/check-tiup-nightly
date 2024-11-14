.PHONY: install run test clean lint help server

# 默认 Python 解释器
PYTHON := python3
# 默认端口
PORT := 5050

FRONTEND_DIR := web
API_BASE_URL ?= http://localhost:5050

help:
	@echo "Available commands:"
	@echo "  make install    - Install required dependencies"
	@echo "  make run       - Run the TiUP checker"
	@echo "  make server    - Run the Flask server"
	@echo "  make lint      - Run code linting"
	@echo "  make clean     - Clean up generated files"
	@echo "  make test      - Run tests"
	@echo "  Frontend commands:"
	@echo "    make frontend-install - Install frontend dependencies"
	@echo "    make frontend-dev    - Run frontend development server"
	@echo "    make frontend-build  - Build frontend for production"
	@echo "    make frontend-clean  - Clean frontend build files"

install:
	$(PYTHON) -m pip install -r requirements.txt

# 添加到现有的 Makefile 中
run:
	API_ENDPOINT=${API_ENDPOINT} $(PYTHON) tiup_checker.py

# 添加一个便捷的本地运行命令
run-local:
	API_ENDPOINT=http://localhost:5050/status $(PYTHON) tiup_checker.py

server:
	FLASK_APP=server.py FLASK_DEBUG=1 $(PYTHON) server.py

lint:
	flake8 *.py
	black *.py

clean:
	find . -type f -name "*.pyc" -delete
	find . -type d -name "__pycache__" -delete
	rm -f tiup_checker.log

test:
	$(PYTHON) -m pytest tests/

# 创建 requirements.txt
requirements:
	$(PYTHON) -m pip freeze > requirements.txt

frontend-install:
	cd $(FRONTEND_DIR) && npm install

frontend-dev:
	cd $(FRONTEND_DIR) && API_BASE_URL=$(API_BASE_URL) && npm run dev

frontend-build:
	cd $(FRONTEND_DIR) && API_BASE_URL=$(API_BASE_URL) && npm run build

frontend-clean:
	rm -rf $(FRONTEND_DIR)/.next
	rm -rf $(FRONTEND_DIR)/node_modules
