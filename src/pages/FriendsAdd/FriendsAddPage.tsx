import { Link } from "react-router-dom";
import { BackIcon } from "@/shared/ui/Icons";

export function FriendsAddPage() {
  return (
    <section className="screen friends-screen">
      <div className="page-header">
        <Link className="icon-button" to="/profile/friends">
          <BackIcon size={17} />
        </Link>
        <h1 className="page-header__title">Друзья</h1>
        <div className="page-header__spacer" />
      </div>

      <div className="card form-grid">
        <div className="card__hint">Ваша ссылка для друзей</div>
        <input value="https://t.me/your_bot?start=friend_123" readOnly />
        <button className="button button--primary" type="button">
          Добавить друзей
        </button>
      </div>
    </section>
  );
}
