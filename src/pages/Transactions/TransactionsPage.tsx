import { Link } from "react-router-dom";
import { BackIcon } from "@/shared/ui/Icons";

const TRANSACTIONS = [
  { id: "t1", type: "Ввод", amount: "+500", channel: "USDT TRC20" },
  { id: "t2", type: "Вывод", amount: "-120", channel: "Карта UAH" },
  { id: "t3", type: "Выигрыш", amount: "+250", channel: "Матч #4821" },
];

export function TransactionsPage() {
  return (
    <section className="screen settings-screen">
      <div className="page-header">
        <Link className="icon-button" to="/profile">
          <BackIcon size={17} />
        </Link>
        <h1 className="page-header__title">История транзакций</h1>
        <div className="page-header__spacer" />
      </div>

      <div className="list">
        {TRANSACTIONS.map((item) => (
          <article className="card" key={item.id}>
            <div className="card__row">
              <strong>{item.type}</strong>
              <strong>{item.amount}</strong>
            </div>
            <div className="card__hint">{item.channel}</div>
          </article>
        ))}
      </div>
    </section>
  );
}
