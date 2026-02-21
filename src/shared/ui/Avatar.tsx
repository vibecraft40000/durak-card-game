type AppAvatarProps = {
  name: string;
  photoUrl?: string;
  className?: string;
};

export function AppAvatar({ name, photoUrl, className = "" }: AppAvatarProps) {
  const letter = (name || "P").slice(0, 1).toUpperCase();
  const hasPhoto = Boolean(photoUrl && String(photoUrl).trim());
  return (
    <div className={`avatar-badge__circle ${className}`.trim()}>
      {hasPhoto ? <img className="avatar-badge__image" src={photoUrl!} alt={name} /> : letter}
    </div>
  );
}
