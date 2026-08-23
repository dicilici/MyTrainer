"""早停策略。"""


class EarlyStopping:
    def __init__(self, patience: int) -> None:
        self.patience = max(0, patience)
        self.best = None
        self.counter = 0
        self.stop = False

    def step(self, metric: float) -> None:
        if self.best is None or metric < self.best:
            self.best = metric
            self.counter = 0
        else:
            self.counter += 1
            if self.counter >= self.patience:
                self.stop = True
